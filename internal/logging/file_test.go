package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileLineas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "dir", "install.log")
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	if err := l.Line(ts, "info", "format", "mkfs.fat -F32 /dev/sda1"); err != nil {
		t.Fatal(err)
	}
	if err := l.Line(ts, "error", "", "primera\nsegunda"); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("líneas = %d, quería 2 (el salto debe escaparse)", len(lines))
	}
	if !strings.Contains(lines[0], "[format] mkfs.fat") {
		t.Errorf("línea 1 inesperada: %q", lines[0])
	}
	if !strings.Contains(lines[1], `primera\nsegunda`) || !strings.Contains(lines[1], "[-]") {
		t.Errorf("línea 2 inesperada: %q", lines[1])
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("permisos = %v, quería 0600", st.Mode().Perm())
	}
}
