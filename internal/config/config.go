// Package config define, carga y valida la configuración de instalación
// (formato descrito en docs/config.md). Solo hace validación estructural:
// lo que depende del sistema (que el disco exista, que el firmware sea UEFI,
// que exista el .desktop) lo comprueba el paso "validate" del motor.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const (
	Version         = 1
	DefaultSwapMiB  = 2048
	DefaultSquashfs = "/run/miso/bootmnt/LiveOS/emancipation-rootfs.squashfs"
	DefaultLogPath  = "/var/log/emancipation-install.log"
)

type Config struct {
	Version    int        `json:"version"`
	Locale     Locale     `json:"locale"`
	Disk       Disk       `json:"disk"`
	Bootloader Bootloader `json:"bootloader"`
	Hostname   string     `json:"hostname"`
	User       User       `json:"user"`
	Desktop    Desktop    `json:"desktop"`
	Source     Source     `json:"source"`
	Options    Options    `json:"options"`
}

type Locale struct {
	Language string `json:"language"`
	Timezone string `json:"timezone"`
	Keymap   string `json:"keymap"`
}

type Disk struct {
	Device       string `json:"device"`
	ConfirmErase string `json:"confirm_erase"`
	SwapMiB      *int   `json:"swap_mib"`
}

type Bootloader struct {
	Target string `json:"target"`
}

type User struct {
	Name      string `json:"name"`
	FullName  string `json:"full_name"`
	Password  string `json:"password"`
	Admin     *bool  `json:"admin"`
	Autologin bool   `json:"autologin"`
}

type Desktop struct {
	Default string `json:"default"`
}

type Source struct {
	Squashfs string `json:"squashfs"`
}

type Options struct {
	DryRun  bool   `json:"dry_run"`
	LogPath string `json:"log_path"`
}

// Problem describe un campo inválido; Field usa la notación del JSON
// ("disk.confirm_erase").
type Problem struct {
	Field   string
	Message string
}

// ValidationError agrupa todos los problemas de una configuración.
// El motor lo reporta como E_CONFIG_INVALID.
type ValidationError struct {
	Problems []Problem
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		parts[i] = p.Field + ": " + p.Message
	}
	return "configuración inválida: " + strings.Join(parts, "; ")
}

var (
	reLanguage = regexp.MustCompile(`^[a-z]{2,3}_[A-Z]{2}(\.[A-Za-z0-9-]+)?$`)
	reTimezone = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+-]*(/[A-Za-z0-9_+-]+)*$`)
	reKeymap   = regexp.MustCompile(`^[a-z0-9_-]+$`)
	reDevice   = regexp.MustCompile(`^/dev/[a-z][a-z0-9]+$`)
	reHostname = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	reUser     = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
)

// Load lee la configuración desde r, rechaza campos desconocidos, aplica los
// valores por defecto y la valida. Un error de validación es *ValidationError.
func Load(r io.Reader) (*Config, error) {
	data, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("leyendo la configuración: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var c Config
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("JSON inválido: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("JSON inválido: hay datos después del objeto")
	}
	c.applyDefaults()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) applyDefaults() {
	if c.Disk.SwapMiB == nil {
		v := DefaultSwapMiB
		c.Disk.SwapMiB = &v
	}
	if c.Bootloader.Target == "" {
		c.Bootloader.Target = "auto"
	}
	if c.User.Admin == nil {
		v := true
		c.User.Admin = &v
	}
	if c.Source.Squashfs == "" {
		c.Source.Squashfs = DefaultSquashfs
	}
	if c.Options.LogPath == "" {
		c.Options.LogPath = DefaultLogPath
	}
}

// Swap devuelve el tamaño del swap en MiB (0 lo desactiva).
func (c *Config) Swap() int {
	if c.Disk.SwapMiB == nil {
		return DefaultSwapMiB
	}
	return *c.Disk.SwapMiB
}

// IsAdmin indica si el usuario entra al grupo wheel (por defecto sí).
func (c *Config) IsAdmin() bool {
	if c.User.Admin == nil {
		return true
	}
	return *c.User.Admin
}

// Redacted devuelve una copia sin la contraseña, segura para el log.
func (c Config) Redacted() Config {
	if c.User.Password != "" {
		c.User.Password = "********"
	}
	return c
}

// Validate comprueba la configuración y devuelve *ValidationError con todos
// los problemas encontrados, o nil si es correcta.
func (c *Config) Validate() error {
	var ps []Problem
	add := func(field, msg string) { ps = append(ps, Problem{Field: field, Message: msg}) }

	if c.Version != Version {
		add("version", fmt.Sprintf("debe ser %d", Version))
	}
	if !reLanguage.MatchString(c.Locale.Language) {
		add("locale.language", "formato esperado: es_ES.UTF-8")
	}
	if !reTimezone.MatchString(c.Locale.Timezone) {
		add("locale.timezone", "zona IANA esperada, por ejemplo America/Bogota")
	}
	if !reKeymap.MatchString(c.Locale.Keymap) {
		add("locale.keymap", "teclado esperado, por ejemplo es")
	}
	if !reDevice.MatchString(c.Disk.Device) {
		add("disk.device", "ruta de disco esperada, por ejemplo /dev/sda")
	}
	if c.Disk.ConfirmErase != c.Disk.Device {
		add("disk.confirm_erase", "debe ser idéntico a disk.device")
	}
	if s := c.Swap(); s < 0 || s > 131072 {
		add("disk.swap_mib", "debe estar entre 0 y 131072")
	}
	switch c.Bootloader.Target {
	case "", "auto", "uefi", "bios", "hybrid":
	default:
		add("bootloader.target", "debe ser auto, uefi, bios o hybrid")
	}
	if !reHostname.MatchString(c.Hostname) {
		add("hostname", "minúsculas, números y guiones, sin espacios (RFC 1123)")
	}
	if !reUser.MatchString(c.User.Name) {
		add("user.name", "formato esperado: ^[a-z_][a-z0-9_-]{0,31}$")
	} else if c.User.Name == "root" {
		add("user.name", "no puede ser root")
	}
	if c.User.Password == "" {
		add("user.password", "no puede estar vacía")
	} else if strings.ContainsAny(c.User.Password, "\r\n") {
		add("user.password", "no puede contener saltos de línea")
	}
	switch c.Desktop.Default {
	case "sway", "niri":
	default:
		add("desktop.default", "debe ser sway o niri")
	}
	if c.Source.Squashfs != "" && !strings.HasPrefix(c.Source.Squashfs, "/") {
		add("source.squashfs", "debe ser una ruta absoluta")
	}
	if c.Options.LogPath != "" && !strings.HasPrefix(c.Options.LogPath, "/") {
		add("options.log_path", "debe ser una ruta absoluta")
	}

	if len(ps) > 0 {
		return &ValidationError{Problems: ps}
	}
	return nil
}
