// Package logging escribe el log persistente de la instalación.
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// File es un log de texto que hace fsync en cada línea, para que un cuelgue o
// un reinicio no pierda el final (en el live /var/log vive en RAM).
type File struct {
	mu sync.Mutex
	f  *os.File
}

// Open abre (o crea) el archivo de log, creando antes el directorio si falta.
func Open(path string) (*File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creando el directorio del log: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("abriendo el log: %w", err)
	}
	return &File{f: f}, nil
}

// Path devuelve la ruta del archivo.
func (l *File) Path() string { return l.f.Name() }

// Line escribe una línea con marca de tiempo y la sincroniza a disco. Los
// saltos de línea del mensaje se escapan para mantener una línea por entrada.
func (l *File) Line(ts time.Time, level, step, msg string) error {
	if step == "" {
		step = "-"
	}
	msg = strings.ReplaceAll(strings.ReplaceAll(msg, "\r", ""), "\n", `\n`)
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := fmt.Fprintf(l.f, "%s %-5s [%s] %s\n", ts.Format(time.RFC3339), level, step, msg); err != nil {
		return err
	}
	return l.f.Sync()
}

// Close cierra el archivo.
func (l *File) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}
