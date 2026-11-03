package config

import (
	"errors"
	"strings"
	"testing"
)

const validJSON = `{
  "version": 1,
  "locale": { "language": "es_ES.UTF-8", "timezone": "America/Bogota", "keymap": "es" },
  "disk": { "device": "/dev/sda", "confirm_erase": "/dev/sda" },
  "hostname": "emancipation-pc",
  "user": { "name": "usuario", "password": "secreto" },
  "desktop": { "default": "sway" }
}`

func mutate(t *testing.T, old, repl string) string {
	t.Helper()
	if !strings.Contains(validJSON, old) {
		t.Fatalf("el texto %q no está en validJSON", old)
	}
	return strings.Replace(validJSON, old, repl, 1)
}

func hasProblem(err error, field string) bool {
	var ve *ValidationError
	if !errors.As(err, &ve) {
		return false
	}
	for _, p := range ve.Problems {
		if p.Field == field {
			return true
		}
	}
	return false
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load(strings.NewReader(validJSON))
	if err != nil {
		t.Fatalf("configuración válida rechazada: %v", err)
	}
	if c.Swap() != DefaultSwapMiB {
		t.Errorf("swap = %d, quería %d", c.Swap(), DefaultSwapMiB)
	}
	if c.Bootloader.Target != "auto" {
		t.Errorf("target = %q, quería auto", c.Bootloader.Target)
	}
	if !c.IsAdmin() {
		t.Error("admin debería ser true por defecto")
	}
	if c.Source.Squashfs != DefaultSquashfs {
		t.Errorf("squashfs = %q", c.Source.Squashfs)
	}
	if c.Options.LogPath != DefaultLogPath {
		t.Errorf("log_path = %q", c.Options.LogPath)
	}
	if c.Options.DryRun {
		t.Error("dry_run debería ser false por defecto")
	}
}

func TestSwapZeroDesactivaElSwap(t *testing.T) {
	js := mutate(t, `"confirm_erase": "/dev/sda" }`, `"confirm_erase": "/dev/sda", "swap_mib": 0 }`)
	c, err := Load(strings.NewReader(js))
	if err != nil {
		t.Fatal(err)
	}
	if c.Swap() != 0 {
		t.Errorf("swap = %d, quería 0", c.Swap())
	}
}

func TestInvalidas(t *testing.T) {
	casos := []struct {
		nombre string
		json   string
		campo  string
	}{
		{"confirmación distinta", mutate(t, `"confirm_erase": "/dev/sda"`, `"confirm_erase": "/dev/sdb"`), "disk.confirm_erase"},
		{"hostname con espacios", mutate(t, `emancipation-pc`, `mi pc`), "hostname"},
		{"usuario root", mutate(t, `"name": "usuario"`, `"name": "root"`), "user.name"},
		{"contraseña vacía", mutate(t, `"secreto"`, `""`), "user.password"},
		{"contraseña con salto de línea", mutate(t, `"secreto"`, `"se\ncreto"`), "user.password"},
		{"escritorio desconocido", mutate(t, `"sway"`, `"gnome"`), "desktop.default"},
		{"versión futura", mutate(t, `"version": 1`, `"version": 2`), "version"},
		{"cargador inválido", mutate(t, `"hostname"`, `"bootloader": {"target": "pxe"}, "hostname"`), "bootloader.target"},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			_, err := Load(strings.NewReader(tc.json))
			if err == nil {
				t.Fatal("se esperaba un error")
			}
			if !hasProblem(err, tc.campo) {
				t.Fatalf("falta el problema en %q: %v", tc.campo, err)
			}
		})
	}
}

func TestCampoDesconocido(t *testing.T) {
	_, err := Load(strings.NewReader(mutate(t, `"hostname"`, `"extra": 1, "hostname"`)))
	if err == nil {
		t.Fatal("un campo desconocido debería rechazarse")
	}
}

func TestDatosSobrantes(t *testing.T) {
	if _, err := Load(strings.NewReader(validJSON + " {}")); err == nil {
		t.Fatal("datos después del objeto deberían rechazarse")
	}
}

func TestRedacted(t *testing.T) {
	c, err := Load(strings.NewReader(validJSON))
	if err != nil {
		t.Fatal(err)
	}
	r := c.Redacted()
	if r.User.Password == "secreto" {
		t.Error("la copia redactada conserva la contraseña")
	}
	if c.User.Password != "secreto" {
		t.Error("Redacted no debe modificar el original")
	}
}
