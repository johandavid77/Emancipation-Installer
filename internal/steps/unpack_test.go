package steps

import (
	"reflect"
	"testing"
)

func TestUnpackCmds(t *testing.T) {
	got := strs(UnpackCmds("/run/miso/bootmnt/LiveOS/rootfs.squashfs", "/mnt/target"))
	want := []string{"unsquashfs -f -percentage -d /mnt/target /run/miso/bootmnt/LiveOS/rootfs.squashfs"}
	iguales(t, got, want)
}

func TestParsePercent(t *testing.T) {
	casos := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"0", 0, true},
		{"42", 42, true},
		{" 42% ", 42, true},
		{"42.5", 42.5, true},
		{"100", 100, true},
		{"101", 0, false},
		{"-1", 0, false},
		{"abc", 0, false},
		{"", 0, false},
		{"12 inodes", 0, false},
		{".", 0, false},
		{"1.2.3", 0, false},
	}
	for _, c := range casos {
		got, ok := ParsePercent(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("ParsePercent(%q) = (%v, %v), quería (%v, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestOnLineRecibeLaSalida(t *testing.T) {
	var got []string
	r := &ExecRunner{Log: &fakeLog{}, OnLine: func(step, line string) {
		got = append(got, step+"|"+line)
	}}
	if err := r.Run("unpack", sh("echo 10; echo 20")); err != nil {
		t.Fatal(err)
	}
	if want := []string{"unpack|10", "unpack|20"}; !reflect.DeepEqual(got, want) {
		t.Errorf("líneas = %v, quería %v", got, want)
	}
}

func TestProgresoDeUnsquashfsSimulado(t *testing.T) {
	var got []float64
	r := &ExecRunner{Log: &fakeLog{}, OnLine: func(step, line string) {
		if p, ok := ParsePercent(line); ok && step == "unpack" {
			got = append(got, p)
		}
	}}
	err := r.Run("unpack", sh("echo 'Parallel unsquashfs: Using 3 processors'; echo 0; echo 50; echo '103519 files'; echo 100"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []float64{0, 50, 100}; !reflect.DeepEqual(got, want) {
		t.Errorf("porcentajes = %v, quería %v", got, want)
	}
}
