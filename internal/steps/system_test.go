package steps

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/johandavid77/Emancipation-Installer/internal/config"
	"github.com/johandavid77/Emancipation-Installer/internal/events"
)

func mkdirAll(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	mkdirAll(t, filepath.Dir(p))
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fakeEnv(t *testing.T) (Env, string) {
	t.Helper()
	root := t.TempDir()
	e := Env{
		SysBlock: filepath.Join(root, "sys/block"),
		SysClass: filepath.Join(root, "sys/class/block"),
		Mounts:   filepath.Join(root, "proc/mounts"),
		Swaps:    filepath.Join(root, "proc/swaps"),
		LabelDir: filepath.Join(root, "dev/disk/by-label"),
		EFIDir:   filepath.Join(root, "sys/firmware/efi"),
	}
	for _, d := range []string{e.SysBlock, e.SysClass, e.LabelDir} {
		mkdirAll(t, d)
	}
	return e, root
}

// fakeLive simula que el medio live está en la partición part del disco disk.
func fakeLive(t *testing.T, e Env, root, disk, part string) {
	t.Helper()
	devFile := filepath.Join(root, "dev", part)
	writeFile(t, devFile, "")
	pdir := filepath.Join(root, "sys/devices/block", disk, part)
	writeFile(t, filepath.Join(pdir, "partition"), "1")
	if err := os.Symlink(pdir, filepath.Join(e.SysClass, part)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(devFile, filepath.Join(e.LabelDir, LiveLabel)); err != nil {
		t.Fatal(err)
	}
}

func TestIsPartOf(t *testing.T) {
	casos := []struct {
		disk, src string
		want      bool
	}{
		{"/dev/sda", "/dev/sda", true},
		{"/dev/sda", "/dev/sda1", true},
		{"/dev/sda", "/dev/sda12", true},
		{"/dev/sda", "/dev/sdb1", false},
		{"/dev/sda", "/dev/sdaa1", false},
		{"/dev/sda", "/dev/sda1x", false},
		{"/dev/nvme0n1", "/dev/nvme0n1p2", true},
		{"/dev/nvme0n1", "/dev/nvme0n1", true},
		{"/dev/nvme0n1", "/dev/nvme0n10p1", false},
		{"/dev/nvme0n1", "/dev/nvme0n12", false},
		{"/dev/sda", "tmpfs", false},
		{"", "/dev/sda1", false},
	}
	for _, c := range casos {
		if got := isPartOf(c.disk, c.src); got != c.want {
			t.Errorf("isPartOf(%q, %q) = %v, quería %v", c.disk, c.src, got, c.want)
		}
	}
}

func TestIsWholeDisk(t *testing.T) {
	e, _ := fakeEnv(t)
	mkdirAll(t, filepath.Join(e.SysBlock, "sda"))
	if !e.IsWholeDisk("/dev/sda") {
		t.Error("/dev/sda debería ser un disco entero")
	}
	if e.IsWholeDisk("/dev/sda1") || e.IsWholeDisk("/etc/passwd") || e.IsWholeDisk("sda") {
		t.Error("una partición o una ruta ajena no son discos enteros")
	}
}

func TestBusy(t *testing.T) {
	e, _ := fakeEnv(t)
	writeFile(t, e.Mounts, "/dev/sda1 /boot ext4 rw 0 0\n/dev/sdb3 / btrfs rw 0 0\n/dev/sdb3 /mnt btrfs rw 0 0\ntmpfs /tmp tmpfs rw 0 0\n")
	writeFile(t, e.Swaps, "Filename Type Size Used Priority\n/dev/sda2 partition 1024 0 -2\n")
	if got, want := e.Busy("/dev/sda"), []string{"/dev/sda1", "/dev/sda2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Busy(sda) = %v, quería %v", got, want)
	}
	if got, want := e.Busy("/dev/sdb"), []string{"/dev/sdb3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Busy(sdb) = %v, quería %v (sin duplicados)", got, want)
	}
	if got := e.Busy("/dev/sdc"); len(got) != 0 {
		t.Errorf("Busy(sdc) = %v, quería vacío", got)
	}
}

func TestBootDisk(t *testing.T) {
	e, root := fakeEnv(t)
	if got := e.BootDisk(); got != "" {
		t.Errorf("sin etiqueta BootDisk = %q, quería vacío", got)
	}
	fakeLive(t, e, root, "sdb", "sdb1")
	if got := e.BootDisk(); got != "/dev/sdb" {
		t.Errorf("BootDisk = %q, quería /dev/sdb", got)
	}
}

func codes(ps []Problem) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Code)
	}
	return out
}

func entornoListo(t *testing.T) (Env, string, *config.Config) {
	t.Helper()
	e, root := fakeEnv(t)
	mkdirAll(t, filepath.Join(e.SysBlock, "sda"))
	mkdirAll(t, e.EFIDir)
	sq := filepath.Join(root, "rootfs.squashfs")
	writeFile(t, sq, "x")
	c := &config.Config{
		Disk:       config.Disk{Device: "/dev/sda", ConfirmErase: "/dev/sda"},
		Bootloader: config.Bootloader{Target: "uefi"},
		Source:     config.Source{Squashfs: sq},
	}
	return e, root, c
}

func TestPreflightTodoBien(t *testing.T) {
	e, _, c := entornoListo(t)
	if ps := Preflight(c, e); len(ps) != 0 {
		t.Fatalf("no debería haber problemas: %v", ps)
	}
}

func TestPreflightDiscoInexistente(t *testing.T) {
	e, root := fakeEnv(t)
	c := &config.Config{
		Disk:       config.Disk{Device: "/dev/sda", ConfirmErase: "/dev/sda"},
		Bootloader: config.Bootloader{Target: "uefi"},
		Source:     config.Source{Squashfs: filepath.Join(root, "no-existe.squashfs")},
	}
	got := codes(Preflight(c, e))
	want := []string{events.ErrDiskNotFound, events.ErrConfigInvalid, events.ErrConfigInvalid}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("códigos = %v, quería %v", got, want)
	}
}

func TestPreflightDiscoEnUso(t *testing.T) {
	e, _, c := entornoListo(t)
	writeFile(t, e.Mounts, "/dev/sda1 /mnt ext4 rw 0 0\n")
	ps := Preflight(c, e)
	if len(ps) != 1 || ps[0].Code != events.ErrDiskBusy || !strings.Contains(ps[0].Message, "/dev/sda1") {
		t.Fatalf("problemas inesperados: %v", ps)
	}
}

func TestPreflightMedioDeArranque(t *testing.T) {
	e, root, c := entornoListo(t)
	fakeLive(t, e, root, "sda", "sda1")
	ps := Preflight(c, e)
	if len(ps) != 1 || ps[0].Code != events.ErrDiskBusy || !strings.Contains(ps[0].Message, "medio de arranque") {
		t.Fatalf("problemas inesperados: %v", ps)
	}
}

func TestPreflightUEFISinFirmwareUEFI(t *testing.T) {
	e, _, c := entornoListo(t)
	if err := os.Remove(e.EFIDir); err != nil {
		t.Fatal(err)
	}
	ps := Preflight(c, e)
	if len(ps) != 1 || ps[0].Code != events.ErrConfigInvalid || !strings.Contains(ps[0].Message, "BIOS") {
		t.Fatalf("problemas inesperados: %v", ps)
	}
}
