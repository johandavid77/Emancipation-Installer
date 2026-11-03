package steps

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johandavid77/Emancipation-Installer/internal/config"
	"github.com/johandavid77/Emancipation-Installer/internal/events"
)

// LiveLabel es la etiqueta del medio de arranque (el volid de la ISO).
const LiveLabel = "EMANCIPATION_LIVE"

// Env agrupa las rutas del sistema que consultan las comprobaciones, para
// poder sustituirlas por directorios falsos en las pruebas.
type Env struct {
	SysBlock string // /sys/block
	SysClass string // /sys/class/block
	Mounts   string // /proc/self/mounts
	Swaps    string // /proc/swaps
	LabelDir string // /dev/disk/by-label
	EFIDir   string // /sys/firmware/efi
}

// DefaultEnv devuelve las rutas reales del sistema.
func DefaultEnv() Env {
	return Env{
		SysBlock: "/sys/block",
		SysClass: "/sys/class/block",
		Mounts:   "/proc/self/mounts",
		Swaps:    "/proc/swaps",
		LabelDir: "/dev/disk/by-label",
		EFIDir:   "/sys/firmware/efi",
	}
}

// Problem es un fallo de las comprobaciones previas.
type Problem struct {
	Code    string
	Message string
}

// HasEFI indica si el sistema arrancó en modo UEFI.
func (e Env) HasEFI() bool {
	_, err := os.Stat(e.EFIDir)
	return err == nil
}

// IsWholeDisk indica si dev (/dev/sda) es un disco entero y no una partición.
func (e Env) IsWholeDisk(dev string) bool {
	if !strings.HasPrefix(dev, "/dev/") {
		return false
	}
	_, err := os.Stat(filepath.Join(e.SysBlock, strings.TrimPrefix(dev, "/dev/")))
	return err == nil
}

// isPartOf indica si src es el disco o una de sus particiones: sda1 es
// partición de sda, pero sdaa1 no; nvme0n1p2 lo es de nvme0n1, y nvme0n10p1 no.
func isPartOf(disk, src string) bool {
	if disk == "" {
		return false
	}
	if src == disk {
		return true
	}
	if !strings.HasPrefix(src, disk) {
		return false
	}
	rest := strings.TrimPrefix(src, disk)
	if last := disk[len(disk)-1]; last >= '0' && last <= '9' {
		if !strings.HasPrefix(rest, "p") {
			return false
		}
		rest = rest[1:]
	}
	if rest == "" {
		return false
	}
	for _, r := range rest {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Busy devuelve los dispositivos del disco que están montados o en uso como swap.
func (e Env) Busy(disk string) []string {
	var out []string
	seen := map[string]bool{}
	for _, file := range []string{e.Mounts, e.Swaps} {
		fh, err := os.Open(file)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(fh)
		for sc.Scan() {
			f := strings.Fields(sc.Text())
			if len(f) > 0 && isPartOf(disk, f[0]) && !seen[f[0]] {
				seen[f[0]] = true
				out = append(out, f[0])
			}
		}
		fh.Close()
	}
	return out
}

// BootDisk devuelve el disco que contiene el medio live (la etiqueta
// LiveLabel), o "" si no se arrancó desde un medio así.
func (e Env) BootDisk() string {
	real, err := filepath.EvalSymlinks(filepath.Join(e.LabelDir, LiveLabel))
	if err != nil {
		return ""
	}
	name := filepath.Base(real)
	p, err := filepath.EvalSymlinks(filepath.Join(e.SysClass, name))
	if err != nil {
		return "/dev/" + name
	}
	if _, err := os.Stat(filepath.Join(p, "partition")); err == nil {
		return "/dev/" + filepath.Base(filepath.Dir(p))
	}
	return "/dev/" + name
}

// Preflight comprueba la configuración contra el sistema y devuelve todos los
// problemas encontrados. En la instalación real cualquiera de ellos es fatal.
func Preflight(c *config.Config, e Env) []Problem {
	var ps []Problem
	add := func(code, msg string) { ps = append(ps, Problem{Code: code, Message: msg}) }

	dev := c.Disk.Device
	if !e.IsWholeDisk(dev) {
		add(events.ErrDiskNotFound, fmt.Sprintf("%s no existe o no es un disco entero", dev))
	} else {
		if busy := e.Busy(dev); len(busy) > 0 {
			add(events.ErrDiskBusy, fmt.Sprintf("%s tiene particiones en uso: %s", dev, strings.Join(busy, ", ")))
		}
		if boot := e.BootDisk(); boot == dev {
			add(events.ErrDiskBusy, fmt.Sprintf("%s es el medio de arranque (%s)", dev, LiveLabel))
		}
	}
	if c.Bootloader.Target == "uefi" && !e.HasEFI() {
		add(events.ErrConfigInvalid, "bootloader.target es uefi pero el sistema arrancó en modo BIOS")
	}
	if _, err := os.Stat(c.Source.Squashfs); err != nil {
		add(events.ErrConfigInvalid, fmt.Sprintf("no se encuentra el sistema de origen %s", c.Source.Squashfs))
	}
	return ps
}
