package steps

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type fakeLog struct {
	mu    sync.Mutex
	lines []string
}

func (f *fakeLog) Log(level, step, msg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lines = append(f.lines, level+"|"+step+"|"+msg)
}

func (f *fakeLog) has(want string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, l := range f.lines {
		if l == want {
			return true
		}
	}
	return false
}

func (f *fakeLog) debug() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, l := range f.lines {
		if strings.HasPrefix(l, "debug|") {
			out = append(out, l[strings.LastIndex(l, "|")+1:])
		}
	}
	return out
}

func sh(script string) Cmd { return Cmd{Args: []string{"sh", "-c", script}} }

func TestExecOK(t *testing.T) {
	l := &fakeLog{}
	r := &ExecRunner{Log: l}
	c := sh("echo hola; echo adios >&2")
	if err := r.Run("format", c); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"info|format|$ " + c.String(), "debug|format|hola", "debug|format|adios"} {
		if !l.has(want) {
			t.Errorf("falta %q en %v", want, l.lines)
		}
	}
}

func TestExecFallaConservaElFinal(t *testing.T) {
	l := &fakeLog{}
	r := &ExecRunner{Log: l}
	err := r.Run("mount", sh("echo uno; echo dos >&2; exit 3"))
	var ce *CmdError
	if !errors.As(err, &ce) {
		t.Fatalf("se esperaba *CmdError, llegó %v", err)
	}
	if ce.Status != 3 {
		t.Errorf("status = %d, quería 3", ce.Status)
	}
	if want := []string{"uno", "dos"}; !reflect.DeepEqual(ce.Tail, want) {
		t.Errorf("tail = %v, quería %v (stdout y stderr en orden)", ce.Tail, want)
	}
	if !strings.Contains(ce.Error(), "código 3") {
		t.Errorf("mensaje inesperado: %s", ce.Error())
	}
}

func TestExecNoExiste(t *testing.T) {
	r := &ExecRunner{Log: &fakeLog{}}
	err := r.Run("format", Cmd{Args: []string{"/no/existe/nada"}})
	var ce *CmdError
	if !errors.As(err, &ce) || ce.Status != -1 {
		t.Fatalf("se esperaba *CmdError con status -1, llegó %v", err)
	}
}

func TestExecLimiteDelFinal(t *testing.T) {
	r := &ExecRunner{Log: &fakeLog{}, TailLines: 5}
	err := r.Run("unpack", sh("seq 1 50; exit 1"))
	var ce *CmdError
	if !errors.As(err, &ce) {
		t.Fatal(err)
	}
	if want := []string{"46", "47", "48", "49", "50"}; !reflect.DeepEqual(ce.Tail, want) {
		t.Errorf("tail = %v, quería %v", ce.Tail, want)
	}
}

func TestExecLineasConRetornoDeCarroYSinSalto(t *testing.T) {
	l := &fakeLog{}
	r := &ExecRunner{Log: l}
	if err := r.Run("unpack", sh(`printf 'a\rb\rc\nsin salto'`)); err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "b", "c", "sin salto"}; !reflect.DeepEqual(l.debug(), want) {
		t.Errorf("líneas = %v, quería %v", l.debug(), want)
	}
}

func TestExecEntorno(t *testing.T) {
	l := &fakeLog{}
	r := &ExecRunner{Log: l, Env: []string{"MI_VAR=hola"}}
	if err := r.Run("system", sh("echo $LC_ALL $MI_VAR")); err != nil {
		t.Fatal(err)
	}
	if !l.has("debug|system|C hola") {
		t.Errorf("entorno inesperado: %v", l.lines)
	}
}

func TestDryRunner(t *testing.T) {
	l := &fakeLog{}
	if err := (DryRunner{Log: l}).Run("mount", cmd("mount", "/dev/sda3", "/mnt/mi destino")); err != nil {
		t.Fatal(err)
	}
	if !l.has("info|mount|[dry-run] mount /dev/sda3 '/mnt/mi destino'") {
		t.Errorf("registro inesperado: %v", l.lines)
	}
}
