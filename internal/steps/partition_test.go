package steps

import "testing"

func TestPartitionCmdsUEFI(t *testing.T) {
	cmds := PartitionCmds("/dev/sda", Layout("/dev/sda", 2048, "uefi"))
	iguales(t, strs(cmds), []string{
		"wipefs --all --force /dev/sda",
		"sfdisk --wipe always --wipe-partitions always /dev/sda",
		"partprobe /dev/sda",
		"udevadm settle",
	})
	want := "label: gpt\n" +
		"size=512MiB, type=C12A7328-F81F-11D2-BA4B-00A0C93EC93B, name=ESP\n" +
		"size=2048MiB, type=0657FD6D-A4AB-43C4-84E5-0933C84B4F4F, name=swap\n" +
		"type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, name=root\n"
	if cmds[1].Stdin != want {
		t.Errorf("esquema inesperado:\n%s\nquería:\n%s", cmds[1].Stdin, want)
	}
}

func TestPartitionCmdsBIOSSinSwap(t *testing.T) {
	cmds := PartitionCmds("/dev/vda", Layout("/dev/vda", 0, "bios"))
	want := "label: gpt\n" +
		"size=1MiB, type=21686148-6449-6E6F-744E-656564454649, name=bios_grub\n" +
		"size=512MiB, type=C12A7328-F81F-11D2-BA4B-00A0C93EC93B, name=ESP\n" +
		"type=0FC63DAF-8483-4772-8E79-3D69D8477DE4, name=root\n"
	if cmds[1].Stdin != want {
		t.Errorf("esquema inesperado:\n%s\nquería:\n%s", cmds[1].Stdin, want)
	}
}
