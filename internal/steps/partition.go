package steps

import (
	"fmt"
	"strings"
)

// GUID de tipo de partición (GPT) que entiende sfdisk.
const (
	guidBIOSBoot  = "21686148-6449-6E6F-744E-656564454649"
	guidEFI       = "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
	guidLinuxSwap = "0657FD6D-A4AB-43C4-84E5-0933C84B4F4F"
	guidLinuxData = "0FC63DAF-8483-4772-8E79-3D69D8477DE4"
)

// sfdiskType traduce el código de tipo de Part (ef02, ef00, 8200, 8300) al GUID.
func sfdiskType(code string) string {
	switch code {
	case "ef02":
		return guidBIOSBoot
	case "ef00":
		return guidEFI
	case "8200":
		return guidLinuxSwap
	default:
		return guidLinuxData
	}
}

// partitionScript genera el esquema que sfdisk lee por la entrada estándar.
// La partición sin tamaño ocupa el resto del disco.
func partitionScript(parts []Part) string {
	var b strings.Builder
	b.WriteString("label: gpt\n")
	for _, p := range parts {
		if p.SizeMiB > 0 {
			fmt.Fprintf(&b, "size=%dMiB, ", p.SizeMiB)
		}
		fmt.Fprintf(&b, "type=%s, name=%s\n", sfdiskType(p.Type), p.Name)
	}
	return b.String()
}

// PartitionCmds borra las firmas del disco y crea la tabla GPT con las
// particiones de Layout, todo en un solo comando sfdisk.
func PartitionCmds(disk string, parts []Part) []Cmd {
	return []Cmd{
		cmd("wipefs", "--all", "--force", disk),
		{
			Args:  []string{"sfdisk", "--wipe", "always", "--wipe-partitions", "always", disk},
			Stdin: partitionScript(parts),
		},
		cmd("partprobe", disk),
		cmd("udevadm", "settle"),
	}
}
