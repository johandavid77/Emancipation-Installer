// Package steps contiene los pasos de la instalación. layout.go calcula la
// disposición del disco (docs/config.md, "Disposición del disco (v1)").
package steps

import "fmt"

// Part describe una partición del disco destino.
type Part struct {
	Num     int
	Name    string // nombre GPT
	Type    string // código de tipo de sgdisk: ef02, ef00, 8200, 8300
	SizeMiB int    // 0 = el resto del disco
	FS      string // fat32, swap, btrfs; vacío si no lleva sistema de archivos
	Label   string
	Mount   string // punto de montaje dentro del destino; vacío si no se monta
	Dev     string // ruta del dispositivo (/dev/sda2, /dev/nvme0n1p2)
}

// Subvol es un subvolumen de la raíz btrfs.
type Subvol struct {
	Name  string
	Mount string
}

// Subvolumes devuelve los subvolúmenes de la raíz, en el orden de montaje.
func Subvolumes() []Subvol {
	return []Subvol{{"@", "/"}, {"@home", "/home"}, {"@snapshots", "/.snapshots"}}
}

// PartPath devuelve la ruta de la partición n de un disco. Los discos cuyo
// nombre termina en dígito (nvme0n1, mmcblk0, loop0) llevan una "p" delante
// del número.
func PartPath(disk string, n int) string {
	if disk != "" {
		if c := disk[len(disk)-1]; c >= '0' && c <= '9' {
			return fmt.Sprintf("%sp%d", disk, n)
		}
	}
	return fmt.Sprintf("%s%d", disk, n)
}

// ResolveTarget convierte "auto" en uefi o bios según el firmware detectado.
func ResolveTarget(target string, efi bool) string {
	if target == "" || target == "auto" {
		if efi {
			return "uefi"
		}
		return "bios"
	}
	return target
}

// Layout calcula las particiones para un target ya resuelto (uefi, bios o
// hybrid). La ESP va siempre; bios_grub solo con bios o hybrid; el swap solo
// si swapMiB es mayor que 0; la raíz btrfs ocupa el resto.
func Layout(disk string, swapMiB int, target string) []Part {
	var parts []Part
	add := func(p Part) {
		p.Num = len(parts) + 1
		p.Dev = PartPath(disk, p.Num)
		parts = append(parts, p)
	}
	if target == "bios" || target == "hybrid" {
		add(Part{Name: "bios_grub", Type: "ef02", SizeMiB: 1})
	}
	add(Part{Name: "ESP", Type: "ef00", SizeMiB: 512, FS: "fat32", Label: "EFI", Mount: "/boot/efi"})
	if swapMiB > 0 {
		add(Part{Name: "swap", Type: "8200", SizeMiB: swapMiB, FS: "swap", Label: "swap"})
	}
	add(Part{Name: "root", Type: "8300", SizeMiB: 0, FS: "btrfs", Label: "emancipation-root", Mount: "/"})
	return parts
}
