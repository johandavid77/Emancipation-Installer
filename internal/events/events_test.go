package events

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type fakeSink struct{ lines []string }

func (s *fakeSink) Line(_ time.Time, level, step, msg string) error {
	s.lines = append(s.lines, level+"|"+step+"|"+msg)
	return nil
}

type harness struct {
	e   *Emitter
	buf *bytes.Buffer
	log *fakeSink
	now time.Time
}

func newHarness() *harness {
	h := &harness{
		buf: &bytes.Buffer{},
		log: &fakeSink{},
		now: time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC),
	}
	h.e = New(h.buf, h.log)
	h.e.now = func() time.Time { return h.now }
	return h
}

func decode(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, ln := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if ln == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("la línea no es JSON: %q: %v", ln, err)
		}
		out = append(out, m)
	}
	return out
}

func TestCamposComunesYOrden(t *testing.T) {
	h := newHarness()
	h.e.Hello("0.1.0", true, "/var/log/x.log")
	h.e.StepStart("validate", true)
	h.e.StepDone("validate", true, 1500*time.Millisecond)

	first := strings.SplitN(h.buf.String(), "\n", 2)[0]
	want := `{"v":1,"ts":"2026-11-03T10:00:00Z","type":"hello"`
	if !strings.HasPrefix(first, want) {
		t.Fatalf("primera línea = %q, debe empezar por %q", first, want)
	}
	evs := decode(t, h.buf)
	tipos := []string{"hello", "step_start", "step_done"}
	if len(evs) != len(tipos) {
		t.Fatalf("eventos = %d, quería %d", len(evs), len(tipos))
	}
	for i, tp := range tipos {
		if evs[i]["type"] != tp || evs[i]["v"] != float64(1) {
			t.Errorf("evento %d = %v", i, evs[i])
		}
	}
	if evs[2]["elapsed_ms"] != float64(1500) {
		t.Errorf("elapsed_ms = %v", evs[2]["elapsed_ms"])
	}
}

func TestProgresoLimitado(t *testing.T) {
	h := newHarness()
	p := func(v float64) *float64 { return &v }
	h.e.StepProgress("unpack", p(10), "a")
	h.now = h.now.Add(100 * time.Millisecond)
	h.e.StepProgress("unpack", p(20), "b") // omitido por el límite
	h.now = h.now.Add(200 * time.Millisecond)
	h.e.StepProgress("unpack", p(30), "c")
	h.now = h.now.Add(10 * time.Millisecond)
	h.e.StepProgress("unpack", p(100), "d") // el 100% siempre se emite
	if n := len(decode(t, h.buf)); n != 3 {
		t.Fatalf("eventos = %d, quería 3", n)
	}
}

func TestProgresoNoMedibleEsNull(t *testing.T) {
	h := newHarness()
	h.e.StepProgress("bootloader", nil, "")
	if !strings.Contains(h.buf.String(), `"percent":null`) {
		t.Fatalf("falta percent null: %s", h.buf.String())
	}
}

func TestSecretosRedactados(t *testing.T) {
	h := newHarness()
	h.e.AddSecret("secreto")
	h.e.Log("info", "user", "chpasswd usuario:secreto")
	h.e.Error("user", ErrUser, true, "falló con secreto", "salida: secreto", "")
	if strings.Contains(h.buf.String(), "secreto") {
		t.Errorf("la contraseña salió en los eventos: %s", h.buf.String())
	}
	for _, l := range h.log.lines {
		if strings.Contains(l, "secreto") {
			t.Errorf("la contraseña salió en el log: %s", l)
		}
	}
}

func TestLogPersiste(t *testing.T) {
	h := newHarness()
	h.e.Log("warn", "format", "mkfs.fat")
	h.e.Log("ruidoso", "format", "nivel desconocido")
	want := []string{"warn|format|mkfs.fat", "info|format|nivel desconocido"}
	if len(h.log.lines) != len(want) {
		t.Fatalf("líneas = %v", h.log.lines)
	}
	for i, w := range want {
		if h.log.lines[i] != w {
			t.Errorf("línea %d = %q, quería %q", i, h.log.lines[i], w)
		}
	}
}

func TestErrorYDone(t *testing.T) {
	h := newHarness()
	h.e.Error("mount", ErrMount, true, "No se pudo montar /dev/sda3", "wrong fs type", "/var/log/x.log")
	h.e.Done(false, false, time.Second)
	evs := decode(t, h.buf)
	if len(evs) != 2 {
		t.Fatalf("eventos = %d, quería 2", len(evs))
	}
	if evs[0]["code"] != "E_MOUNT" || evs[0]["fatal"] != true || evs[0]["log_path"] != "/var/log/x.log" {
		t.Errorf("error inesperado: %v", evs[0])
	}
	if evs[1]["type"] != "done" || evs[1]["success"] != false || evs[1]["elapsed_ms"] != float64(1000) {
		t.Errorf("done inesperado: %v", evs[1])
	}
}

func TestPlan(t *testing.T) {
	if err := ValidatePlan(DefaultPlan()); err != nil {
		t.Fatalf("el plan por defecto es inválido: %v", err)
	}
	if err := ValidatePlan([]Step{{"a", "A", 50}, {"b", "B", 49}}); err == nil {
		t.Error("pesos que suman 99 deberían fallar")
	}
	if err := ValidatePlan([]Step{{"a", "A", 50}, {"a", "B", 50}}); err == nil {
		t.Error("ids repetidos deberían fallar")
	}
}
