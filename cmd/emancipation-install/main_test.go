package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig escribe una configuración válida que apunta a un disco que no
// existe, de modo que ninguna prueba puede tocar un dispositivo real.
func writeConfig(t *testing.T, device, confirm string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := `{
  "version": 1,
  "locale": { "language": "es_ES.UTF-8", "timezone": "America/Bogota", "keymap": "es" },
  "disk": { "device": "` + device + `", "confirm_erase": "` + confirm + `" },
  "hostname": "prueba",
  "user": { "name": "usuario", "password": "secreto" },
  "desktop": { "default": "sway" },
  "options": { "log_path": "` + filepath.Join(dir, "install.log") + `" }
}`
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func ejecutar(t *testing.T, args ...string) (int, []map[string]any, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, strings.NewReader(""), &out, &errb)
	var evs []map[string]any
	for _, ln := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if ln == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("la línea no es JSON: %q: %v", ln, err)
		}
		evs = append(evs, m)
	}
	return code, evs, out.String()
}

func tipo(e map[string]any) string {
	s, _ := e["type"].(string)
	return s
}

func contar(evs []map[string]any, typ string) int {
	n := 0
	for _, e := range evs {
		if tipo(e) == typ {
			n++
		}
	}
	return n
}

func pasoIniciado(evs []map[string]any, id string) bool {
	for _, e := range evs {
		if tipo(e) == "step_start" && e["step"] == id {
			return true
		}
	}
	return false
}

func codigoDeError(evs []map[string]any) string {
	for _, e := range evs {
		if tipo(e) == "error" {
			s, _ := e["code"].(string)
			return s
		}
	}
	return ""
}

func mensajeDeError(evs []map[string]any) string {
	for _, e := range evs {
		if tipo(e) == "error" {
			s, _ := e["message"].(string)
			return s
		}
	}
	return ""
}

func TestPorDefectoEsDryRun(t *testing.T) {
	code, evs, raw := ejecutar(t, "--config", writeConfig(t, "/dev/nodisk0", "/dev/nodisk0"))
	if code != 0 {
		t.Fatalf("código = %d, quería 0", code)
	}
	if tipo(evs[0]) != "hello" || evs[0]["dry_run"] != true {
		t.Errorf("hello inesperado: %v", evs[0])
	}
	if last := evs[len(evs)-1]; tipo(last) != "done" || last["success"] != true {
		t.Errorf("último evento inesperado: %v", last)
	}
	if n := contar(evs, "step_start"); n != 12 {
		t.Errorf("pasos iniciados = %d, quería 12", n)
	}
	if strings.Contains(raw, "secreto") {
		t.Error("la contraseña salió en los eventos")
	}
}

func TestStopAfterEnDryRun(t *testing.T) {
	code, evs, _ := ejecutar(t, "--config", writeConfig(t, "/dev/nodisk0", "/dev/nodisk0"), "--stop-after", "unpack")
	if code != 0 {
		t.Fatalf("código = %d, quería 0", code)
	}
	if n := contar(evs, "step_start"); n != 5 {
		t.Errorf("pasos iniciados = %d, quería 5", n)
	}
	if pasoIniciado(evs, "fstab") {
		t.Error("no debería haber llegado a fstab")
	}
}

func TestStopAfterDesconocido(t *testing.T) {
	code, _, _ := ejecutar(t, "--config", writeConfig(t, "/dev/nodisk0", "/dev/nodisk0"), "--stop-after", "nada")
	if code != 2 {
		t.Errorf("código = %d, quería 2", code)
	}
}

func TestDryRunYExecuteSonIncompatibles(t *testing.T) {
	code, _, _ := ejecutar(t, "--config", writeConfig(t, "/dev/nodisk0", "/dev/nodisk0"), "--dry-run", "--execute")
	if code != 2 {
		t.Errorf("código = %d, quería 2", code)
	}
}

func TestConfigInvalida(t *testing.T) {
	code, evs, _ := ejecutar(t, "--config", writeConfig(t, "/dev/nodisk0", "/dev/nodisk1"))
	if code != 2 {
		t.Errorf("código = %d, quería 2", code)
	}
	if got := codigoDeError(evs); got != "E_CONFIG_INVALID" {
		t.Errorf("código de error = %q", got)
	}
}

func TestExecuteSinStopAfterSeNiegaAntesDeTocarNada(t *testing.T) {
	code, evs, _ := ejecutar(t, "--config", writeConfig(t, "/dev/nodisk0", "/dev/nodisk0"), "--execute")
	if code != 1 {
		t.Fatalf("código = %d, quería 1", code)
	}
	if !strings.Contains(mensajeDeError(evs), "fstab") {
		t.Errorf("el error debería nombrar los pasos sin implementar: %q", mensajeDeError(evs))
	}
	if pasoIniciado(evs, "partition") {
		t.Error("no debería haber iniciado ningún paso")
	}
}

func TestExecuteConStopAfterFallaAntesDelDisco(t *testing.T) {
	code, evs, _ := ejecutar(t, "--config", writeConfig(t, "/dev/nodisk0", "/dev/nodisk0"), "--execute", "--stop-after", "unpack")
	if code == 0 {
		t.Fatal("con un disco inexistente la instalación real no puede terminar bien")
	}
	if codigoDeError(evs) == "" {
		t.Error("debería haber un evento de error")
	}
	if pasoIniciado(evs, "partition") {
		t.Error("no debería haber llegado a particionar")
	}
	if last := evs[len(evs)-1]; tipo(last) != "done" || last["success"] != false {
		t.Errorf("último evento inesperado: %v", last)
	}
}
