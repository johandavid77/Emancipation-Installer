package steps

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Logger es lo que el ejecutor necesita del emisor de eventos.
type Logger interface {
	Log(level, step, msg string)
}

// Executor ejecuta los comandos de un paso.
type Executor interface {
	Run(step string, c Cmd) error
}

// DryRunner no ejecuta nada: solo registra lo que haría.
type DryRunner struct{ Log Logger }

func (d DryRunner) Run(step string, c Cmd) error {
	d.Log.Log("info", step, "[dry-run] "+c.String())
	logStdin(d.Log, step, c, "[dry-run]   < ")
	return nil
}

// CmdError es el error de un comando que no pudo ejecutarse o terminó mal.
type CmdError struct {
	Cmd    string
	Status int      // código de salida; -1 si no llegó a ejecutarse o lo mató una señal
	Tail   []string // últimas líneas de salida
	Err    error
}

func (e *CmdError) Error() string {
	if e.Status < 0 {
		return fmt.Sprintf("no se pudo ejecutar %s: %v", e.Cmd, e.Err)
	}
	return fmt.Sprintf("%s terminó con código %d", e.Cmd, e.Status)
}

func (e *CmdError) Unwrap() error { return e.Err }

// ExecRunner ejecuta los comandos de verdad. Cada línea de salida (stdout y
// stderr juntas, en orden) va al log con nivel debug; el comando mismo, con
// nivel info. Fuerza LC_ALL=C para que la salida sea siempre la misma.
type ExecRunner struct {
	Log       Logger
	Env       []string // variables extra, además del entorno actual
	TailLines int      // líneas finales que se conservan para el error (defecto 20)
}

func (r *ExecRunner) Run(step string, c Cmd) error {
	if len(c.Args) == 0 {
		return &CmdError{Cmd: "(vacío)", Status: -1, Err: errors.New("comando vacío")}
	}
	r.Log.Log("info", step, "$ "+c.String())
	logStdin(r.Log, step, c, "  < ")

	n := r.TailLines
	if n <= 0 {
		n = 20
	}
	tail := &tailBuf{n: n}
	w := &lineWriter{emit: func(s string) {
		r.Log.Log("debug", step, s)
		tail.add(s)
	}}

	cmd := exec.Command(c.Args[0], c.Args[1:]...)
	cmd.Env = append(append(os.Environ(), "LC_ALL=C"), r.Env...)
	if c.Stdin != "" {
		cmd.Stdin = strings.NewReader(c.Stdin)
	}
	cmd.Stdout, cmd.Stderr = w, w // el mismo escritor: un solo canal y el orden se conserva
	err := cmd.Run()
	w.Flush()
	if err == nil {
		return nil
	}
	ce := &CmdError{Cmd: c.String(), Status: -1, Tail: tail.lines, Err: err}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		ce.Status = ee.ExitCode()
	}
	return ce
}

// lineWriter parte lo que recibe en líneas, separando por \n y por \r (así las
// barras de progreso que reescriben la línea salen como líneas separadas).
type lineWriter struct {
	mu   sync.Mutex
	buf  []byte
	emit func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexAny(w.buf, "\r\n")
		if i < 0 {
			break
		}
		line := strings.TrimSpace(string(w.buf[:i]))
		w.buf = w.buf[i+1:]
		if line != "" {
			w.emit(line)
		}
	}
	if len(w.buf) > 1<<20 { // una línea absurdamente larga: se corta
		w.emit(string(w.buf))
		w.buf = nil
	}
	return len(p), nil
}

// Flush emite lo que quede sin salto de línea al final.
func (w *lineWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if s := strings.TrimSpace(string(w.buf)); s != "" {
		w.emit(s)
	}
	w.buf = nil
}

type tailBuf struct {
	n     int
	lines []string
}

func (t *tailBuf) add(s string) {
	t.lines = append(t.lines, s)
	if len(t.lines) > t.n {
		t.lines = t.lines[len(t.lines)-t.n:]
	}
}
