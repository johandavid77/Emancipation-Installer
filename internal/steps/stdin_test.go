package steps

import "testing"

func TestExecConEntrada(t *testing.T) {
	l := &fakeLog{}
	r := &ExecRunner{Log: l}
	c := Cmd{Args: []string{"cat"}, Stdin: "hola\nmundo\n"}
	if err := r.Run("user", c); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"info|user|  < hola", "info|user|  < mundo", "debug|user|hola", "debug|user|mundo"} {
		if !l.has(want) {
			t.Errorf("falta %q en %v", want, l.lines)
		}
	}
}

func TestEntradaSensibleNoSeRegistra(t *testing.T) {
	l := &fakeLog{}
	r := &ExecRunner{Log: l}
	c := Cmd{Args: []string{"sh", "-c", "cat > /dev/null"}, Stdin: "usuario:secreto\n", QuietStdin: true}
	if err := r.Run("user", c); err != nil {
		t.Fatal(err)
	}
	for _, line := range l.lines {
		if len(line) >= 7 && (contiene(line, "secreto") || contiene(line, "usuario:")) {
			t.Errorf("la entrada sensible salió en el log: %q", line)
		}
	}
}

func contiene(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestDryRunnerMuestraLaEntrada(t *testing.T) {
	l := &fakeLog{}
	c := Cmd{Args: []string{"sfdisk", "/dev/sda"}, Stdin: "label: gpt\ntype=L\n"}
	if err := (DryRunner{Log: l}).Run("partition", c); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"info|partition|[dry-run] sfdisk /dev/sda", "info|partition|[dry-run]   < label: gpt", "info|partition|[dry-run]   < type=L"} {
		if !l.has(want) {
			t.Errorf("falta %q en %v", want, l.lines)
		}
	}
}
