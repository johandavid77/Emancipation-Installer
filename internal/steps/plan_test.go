package steps

import (
	"reflect"
	"strings"
	"testing"
)

func strs(cmds []Cmd) []string {
	out := make([]string, len(cmds))
	for i, c := range cmds {
		out[i] = c.String()
	}
	return out
}

func iguales(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("comandos distintos\n--- obtenido\n%s\n--- esperado\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestQuote(t *testing.T) {
	casos := map[string]string{
		"/dev/sda":  "/dev/sda",
		"1:0:+512M": "1:0:+512M",
		"":          "''",
		"mi disco":  "'mi disco'",
		"it's":      `'it'\''s'`,
	}
	for in, want := range casos {
		if got := quote(in); got != want {
			t.Errorf("quote(%q) = %q, quería %q", in, got, want)
		}
	}
}

func TestFormatCmds(t *testing.T) {
	parts := Layout("/dev/sda", 2048, "uefi")
	iguales(t, strs(FormatCmds(parts)), []string{
		"mkfs.fat -F 32 -n EFI /dev/sda1",
		"mkswap -L swap /dev/sda2",
		"mkfs.btrfs -f -L emancipation-root /dev/sda3",
	})
}

func TestMountCmds(t *testing.T) {
	parts := Layout("/dev/sda", 2048, "uefi")
	cmds, err := MountCmds(parts, "/mnt/target")
	if err != nil {
		t.Fatal(err)
	}
	o := BtrfsOpts
	iguales(t, strs(cmds), []string{
		"mkdir -p /mnt/target",
		"mount /dev/sda3 /mnt/target",
		"btrfs subvolume create /mnt/target/@",
		"btrfs subvolume create /mnt/target/@home",
		"btrfs subvolume create /mnt/target/@snapshots",
		"umount /mnt/target",
		"mount -o subvol=@," + o + " /dev/sda3 /mnt/target",
		"mkdir -p /mnt/target/home /mnt/target/.snapshots /mnt/target/boot/efi",
		"mount -o subvol=@home," + o + " /dev/sda3 /mnt/target/home",
		"mount -o subvol=@snapshots," + o + " /dev/sda3 /mnt/target/.snapshots",
		"mount /dev/sda1 /mnt/target/boot/efi",
		"swapon /dev/sda2",
	})
}

func TestMountCmdsSinSwapYErrores(t *testing.T) {
	cmds, err := MountCmds(Layout("/dev/nvme0n1", 0, "uefi"), "/mnt/target")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range strs(cmds) {
		if strings.HasPrefix(c, "swapon") {
			t.Errorf("sin swap no debe haber swapon: %s", c)
		}
	}
	if _, err := MountCmds(nil, "/mnt/target"); err == nil {
		t.Error("sin particiones debería fallar")
	}
}
