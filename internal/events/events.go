// Package events implementa el protocolo de eventos del motor
// (docs/events.md): un objeto JSON por línea.
package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const ProtocolVersion = 1

// Códigos de error (docs/events.md).
const (
	ErrConfigInvalid = "E_CONFIG_INVALID"
	ErrDiskNotFound  = "E_DISK_NOT_FOUND"
	ErrDiskBusy      = "E_DISK_BUSY"
	ErrPartition     = "E_PARTITION"
	ErrFormat        = "E_FORMAT"
	ErrMount         = "E_MOUNT"
	ErrUnpack        = "E_UNPACK"
	ErrSystem        = "E_SYSTEM"
	ErrUser          = "E_USER"
	ErrBootloader    = "E_BOOTLOADER"
	ErrInternal      = "E_INTERNAL"
)

// Códigos de salida del proceso.
const (
	ExitOK            = 0
	ExitStepFailed    = 1
	ExitConfigInvalid = 2
	ExitCancelled     = 130
)

// Step es un paso del plan; Weight es su peso en el avance total.
type Step struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Weight int    `json:"weight"`
}

// DefaultPlan devuelve los pasos de la instalación v1; los pesos suman 100.
func DefaultPlan() []Step {
	return []Step{
		{"validate", "Comprobando la configuración", 1},
		{"partition", "Particionando el disco", 3},
		{"format", "Creando sistemas de archivos", 3},
		{"mount", "Montando el destino", 1},
		{"unpack", "Copiando el sistema", 70},
		{"fstab", "Generando fstab", 1},
		{"system", "Configurando idioma, teclado y hora", 4},
		{"user", "Creando el usuario", 2},
		{"desktop", "Configurando el escritorio", 2},
		{"bootloader", "Instalando el cargador de arranque", 8},
		{"services", "Activando servicios", 2},
		{"finish", "Desmontando y terminando", 3},
	}
}

// ValidatePlan comprueba ids únicos, pesos positivos y suma 100.
func ValidatePlan(steps []Step) error {
	sum := 0
	seen := map[string]bool{}
	for _, s := range steps {
		if s.ID == "" {
			return errors.New("paso sin id")
		}
		if seen[s.ID] {
			return fmt.Errorf("id repetido: %s", s.ID)
		}
		seen[s.ID] = true
		if s.Weight <= 0 {
			return fmt.Errorf("el paso %s tiene un peso no positivo", s.ID)
		}
		sum += s.Weight
	}
	if sum != 100 {
		return fmt.Errorf("los pesos suman %d, deben sumar 100", sum)
	}
	return nil
}

// LogSink recibe cada línea de log para persistirla (ver internal/logging).
type LogSink interface {
	Line(ts time.Time, level, step, msg string) error
}

// Header son los campos comunes de todo evento.
type Header struct {
	V    int    `json:"v"`
	TS   string `json:"ts"`
	Type string `json:"type"`
}

const progressInterval = 250 * time.Millisecond

// Emitter escribe eventos JSON Lines. Es seguro entre goroutines.
type Emitter struct {
	mu       sync.Mutex
	out      io.Writer
	sink     LogSink
	now      func() time.Time
	secrets  []string
	lastProg map[string]time.Time
	werr     error
}

// New crea un emisor que escribe en out y persiste los logs en sink (puede ser nil).
func New(out io.Writer, sink LogSink) *Emitter {
	return &Emitter{out: out, sink: sink, now: time.Now, lastProg: map[string]time.Time{}}
}

// AddSecret registra un texto (la contraseña) que nunca debe salir en un evento ni en el log.
func (e *Emitter) AddSecret(s string) {
	if s == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.secrets = append(e.secrets, s)
}

// WriteErr devuelve el primer error al escribir en out (por ejemplo, si la
// interfaz se cerró). La instalación continúa aunque ocurra.
func (e *Emitter) WriteErr() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.werr
}

func (e *Emitter) head(typ string) Header {
	return Header{V: ProtocolVersion, TS: e.now().Format(time.RFC3339), Type: typ}
}

func (e *Emitter) redact(s string) string {
	for _, sec := range e.secrets {
		s = strings.ReplaceAll(s, sec, "********")
	}
	return s
}

func (e *Emitter) write(v any) {
	data, err := json.Marshal(v)
	if err == nil {
		_, err = e.out.Write(append(data, '\n'))
	}
	if err != nil && e.werr == nil {
		e.werr = err
	}
}

func (e *Emitter) persist(level, step, msg string) {
	if e.sink != nil {
		_ = e.sink.Line(e.now(), level, step, msg)
	}
}

func normLevel(l string) string {
	switch l {
	case "debug", "info", "warn", "error":
		return l
	}
	return "info"
}

func clamp(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

func (e *Emitter) Hello(engine string, dryRun bool, logPath string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.write(struct {
		Header
		Engine  string `json:"engine"`
		DryRun  bool   `json:"dry_run"`
		LogPath string `json:"log_path"`
	}{e.head("hello"), engine, dryRun, logPath})
}

func (e *Emitter) Plan(steps []Step) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.write(struct {
		Header
		Steps []Step `json:"steps"`
	}{e.head("plan"), steps})
}

func (e *Emitter) StepStart(step string, cancelAllowed bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.write(struct {
		Header
		Step          string `json:"step"`
		CancelAllowed bool   `json:"cancel_allowed"`
	}{e.head("step_start"), step, cancelAllowed})
}

// StepProgress emite el avance de un paso (percent nil = no medible). Se
// limita a 4 eventos por segundo y paso; 0 y 100 siempre se emiten.
func (e *Emitter) StepProgress(step string, percent *float64, detail string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if percent != nil {
		v := clamp(*percent)
		percent = &v
	}
	now := e.now()
	final := percent != nil && (*percent <= 0 || *percent >= 100)
	if last, ok := e.lastProg[step]; ok && !final && now.Sub(last) < progressInterval {
		return
	}
	e.lastProg[step] = now
	e.write(struct {
		Header
		Step    string   `json:"step"`
		Percent *float64 `json:"percent"`
		Detail  string   `json:"detail,omitempty"`
	}{e.head("step_progress"), step, percent, e.redact(detail)})
}

func (e *Emitter) StepDone(step string, ok bool, elapsed time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.write(struct {
		Header
		Step      string `json:"step"`
		OK        bool   `json:"ok"`
		ElapsedMS int64  `json:"elapsed_ms"`
	}{e.head("step_done"), step, ok, elapsed.Milliseconds()})
}

// Log emite una línea de log y la persiste. level: debug, info, warn o error.
func (e *Emitter) Log(level, step, msg string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	level = normLevel(level)
	msg = e.redact(msg)
	e.write(struct {
		Header
		Level   string `json:"level"`
		Step    string `json:"step,omitempty"`
		Message string `json:"message"`
	}{e.head("log"), level, step, msg})
	e.persist(level, step, msg)
}

// Warning emite un aviso no bloqueante.
func (e *Emitter) Warning(step, code, msg string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	msg = e.redact(msg)
	e.write(struct {
		Header
		Step    string `json:"step"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}{e.head("warning"), step, code, msg})
	e.persist("warn", step, code+": "+msg)
}

// Error emite un error; tras uno fatal el motor debe emitir Done(false, ...).
func (e *Emitter) Error(step, code string, fatal bool, msg, detail, logPath string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	msg, detail = e.redact(msg), e.redact(detail)
	e.write(struct {
		Header
		Step    string `json:"step"`
		Code    string `json:"code"`
		Fatal   bool   `json:"fatal"`
		Message string `json:"message"`
		Detail  string `json:"detail,omitempty"`
		LogPath string `json:"log_path,omitempty"`
	}{e.head("error"), step, code, fatal, msg, detail, logPath})
	line := code + ": " + msg
	if detail != "" {
		line += " | " + detail
	}
	e.persist("error", step, line)
}

// Done es el último evento, siempre.
func (e *Emitter) Done(success, rebootRequired bool, elapsed time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.write(struct {
		Header
		Success        bool  `json:"success"`
		RebootRequired bool  `json:"reboot_required"`
		ElapsedMS      int64 `json:"elapsed_ms"`
	}{e.head("done"), success, rebootRequired, elapsed.Milliseconds()})
}
