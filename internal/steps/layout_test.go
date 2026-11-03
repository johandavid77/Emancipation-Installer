package steps

import "testing"

func TestPartPath(t *testing.T) {
	casos := []struct {
		disk string
		n    int
		want string
	}{
		{"/dev/sda", 1, "/dev/sda1"},
		{"/dev/vda", 3, "/dev/vda3"},
		{"/dev/nvme0n1", 2, "/dev/nvme0n1p2"},
		{"/dev/mmcblk0", 1, "/dev/mmcblk0p1"},
		{"/dev/loop0", 4, "/dev/loop0p4"},
	}
	for _, c := range casos {
		if got := PartPath(c.disk, c.n); got != c.want {
			t.Errorf("PartPath(%q, %d) = %q, quería %q", c.disk, c.n, got, c.want)
		}
	}
}

func TestResolveTarget(t *testing.T) {
	casos := []struct {
		target string
		efi    bool
		want   string
	}{
		{"auto", true, "uefi"},
		{"auto", false, "bios"},
		{"", true, "uefi"},
		{"uefi", false, "uefi"},
		{"hybrid", true, "hybrid"},
	}
	for _, c := range casos {
		if got := ResolveTarget(c.target, c.efi); got != c.want {
			t.Errorf("ResolveTarget(%q, %v) = %q, quería %q", c.target, c.efi, got, c.want)
		}
	}
}

type esperada struct {
	name, dev, fs, mount string
	size                 int
}

func comparar(t *testing.T, got []Part, want []esperada) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("particiones = %d, quería %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		g := got[i]
		if g.Num != i+1 || g.Name != w.name || g.Dev != w.dev || g.FS != w.fs || g.Mount != w.mount || g.SizeMiB != w.size {
			t.Errorf("partición %d = %+v, quería %+v", i+1, g, w)
		}
	}
}

func TestLayoutUEFIConSwap(t *testing.T) {
	comparar(t, Layout("/dev/sda", 2048, "uefi"), []esperada{
		{"ESP", "/dev/sda1", "fat32", "/boot/efi", 512},
		{"swap", "/dev/sda2", "swap", "", 2048},
		{"root", "/dev/sda3", "btrfs", "/", 0},
	})
}

func TestLayoutHybridSinSwap(t *testing.T) {
	comparar(t, Layout("/dev/nvme0n1", 0, "hybrid"), []esperada{
		{"bios_grub", "/dev/nvme0n1p1", "", "", 1},
		{"ESP", "/dev/nvme0n1p2", "fat32", "/boot/efi", 512},
		{"root", "/dev/nvme0n1p3", "btrfs", "/", 0},
	})
}

func TestLayoutBIOSConSwap(t *testing.T) {
	comparar(t, Layout("/dev/vda", 1024, "bios"), []esperada{
		{"bios_grub", "/dev/vda1", "", "", 1},
		{"ESP", "/dev/vda2", "fat32", "/boot/efi", 512},
		{"swap", "/dev/vda3", "swap", "", 1024},
		{"root", "/dev/vda4", "btrfs", "/", 0},
	})
}

func TestSubvolumes(t *testing.T) {
	want := []Subvol{{"@", "/"}, {"@home", "/home"}, {"@snapshots", "/.snapshots"}}
	got := Subvolumes()
	if len(got) != len(want) {
		t.Fatalf("subvolúmenes = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("subvolumen %d = %v, quería %v", i, got[i], want[i])
		}
	}
}
