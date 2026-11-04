// Command emancipation-install es el motor de instalación: lee la
// configuración, ejecuta los pasos y emite eventos JSON Lines por stdout
// (docs/events.md). Sin --execute solo describe lo que haría (dry-run).
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/johandavid77/Emancipation-Installer/internal/config"
	"github.com/johandavid77/Emancipation-Installer/internal/events"
	"github.com/johandavid77/Emancipation-Installer/internal/logging"
	"github.com/johandavid77/Emancipation-Installer/internal/steps"
)

const (
	engineVersion = "0.1.0"
	targetDir     = "/mnt/emancipation-target"
)

// implemented lista los pasos que el motor sabe ejecutar de verdad.
var implemented = map[string]bool{
	"validate": true, "partition": true, "format": true, "mount": true, "unpack": true,
}

// stepError asocia cada paso con el código de error de su fallo.
var stepError = map[string]string{
	"partition": events.ErrPartition,
	"format":    events.ErrFormat,
	"mount":     events.ErrMount,
	"unpack":    events.ErrUnpack,
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func describe(p steps.Part) string {
	size := "resto del disco"
	if p.SizeMiB > 0 {
		size = fmt.Sprintf("%d MiB", p.SizeMiB)
	}
	fsName, mnt := p.FS, p.Mount
	if fsName == "" {
		fsName = "-"
	}
	if mnt == "" {
		mnt = "-"
	}
	return fmt.Sprintf("%-16s %-9s %-16s %-6s %s", p.Dev, p.Name, size, fsName, mnt)
}

// buildCmds devuelve los comandos de un paso; known es false si el paso aún
// no está implementado.
func buildCmds(id string, cfg *config.Config, parts []steps.Part) (cmds []steps.Cmd, known bool, err error) {
	switch id {
	case "partition":
		return steps.PartitionCmds(cfg.Disk.Device, parts), true, nil
	case "format":
		return steps.FormatCmds(parts), true, nil
	case "mount":
		cmds, err = steps.MountCmds(parts, targetDir)
		return cmds, true, err
	case "unpack":
		return steps.UnpackCmds(cfg.Source.Squashfs, targetDir), true, nil
	}
	return nil, false, nil
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	start := time.Now()
	fs := flag.NewFlagSet("emancipation-install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "", "archivo de configuración JSON ('-' para stdin)")
	dryFlag := fs.Bool("dry-run", false, "solo describir lo que se haría (es el modo por defecto)")
	execFlag := fs.Bool("execute", false, "instalar de verdad; sin esta opción el motor solo describe")
	stopAfter := fs.String("stop-after", "", "detenerse después de este paso (para pruebas)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return events.ExitOK
		}
		return events.ExitConfigInvalid
	}
	if *cfgPath == "" {
		fmt.Fprintln(stderr, "falta --config <archivo>")
		return events.ExitConfigInvalid
	}
	if *dryFlag && *execFlag {
		fmt.Fprintln(stderr, "--dry-run y --execute son incompatibles")
		return events.ExitConfigInvalid
	}

	plan := events.DefaultPlan()
	last := len(plan) - 1
	if *stopAfter != "" {
		last = -1
		for i, s := range plan {
			if s.ID == *stopAfter {
				last = i
			}
		}
		if last < 0 {
			fmt.Fprintf(stderr, "paso desconocido en --stop-after: %s\n", *stopAfter)
			return events.ExitConfigInvalid
		}
	}

	in := stdin
	if *cfgPath != "-" {
		f, err := os.Open(*cfgPath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return events.ExitConfigInvalid
		}
		defer f.Close()
		in = f
	}

	cfg, err := config.Load(in)
	if err != nil {
		em := events.New(stdout, nil)
		em.Hello(engineVersion, !*execFlag, config.DefaultLogPath)
		em.Error("validate", events.ErrConfigInvalid, true, err.Error(), "", "")
		em.Done(false, false, time.Since(start))
		return events.ExitConfigInvalid
	}
	dry := *dryFlag || cfg.Options.DryRun || !*execFlag

	var sink events.LogSink
	lf, lerr := logging.Open(cfg.Options.LogPath)
	if lerr == nil {
		sink = lf
		defer lf.Close()
	}
	em := events.New(stdout, sink)
	em.AddSecret(cfg.User.Password)
	em.Hello(engineVersion, dry, cfg.Options.LogPath)

	fail := func(exit int, step, code, msg, detail string) int {
		em.Error(step, code, true, msg, detail, cfg.Options.LogPath)
		em.Done(false, false, time.Since(start))
		return exit
	}
	if lerr != nil {
		if !dry {
			return fail(events.ExitStepFailed, "validate", events.ErrInternal, "no se pudo abrir el log de la instalación", lerr.Error())
		}
		em.Warning("validate", "W_LOG", "no se pudo abrir el log ("+lerr.Error()+"); el dry-run continúa sin archivo")
	}

	// Barreras de la instalación real: todas se comprueban antes de tocar nada.
	if !dry {
		var missing []string
		for _, s := range plan[:last+1] {
			if !implemented[s.ID] {
				missing = append(missing, s.ID)
			}
		}
		if len(missing) > 0 {
			return fail(events.ExitStepFailed, "validate", events.ErrInternal,
				"la instalación real aún no puede completar estos pasos: "+strings.Join(missing, ", ")+" (use --stop-after para detenerse antes)", "")
		}
		if os.Geteuid() != 0 {
			return fail(events.ExitStepFailed, "validate", events.ErrInternal, "la instalación real necesita permisos de root", "")
		}
	}

	// Una señal solo cancela mientras el paso validate lo permite.
	var cancelled atomic.Bool
	sigs := make(chan os.Signal, 1)
	quit := make(chan struct{})
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigs)
	defer close(quit)
	go func() {
		for {
			select {
			case <-sigs:
				cancelled.Store(true)
			case <-quit:
				return
			}
		}
	}()

	var ex steps.Executor
	if dry {
		ex = steps.DryRunner{Log: em}
	} else {
		ex = &steps.ExecRunner{Log: em, OnLine: func(step, line string) {
			if step != "unpack" {
				return
			}
			if p, ok := steps.ParsePercent(line); ok {
				em.StepProgress(step, &p, "")
			}
		}}
	}
	pre := ""
	if dry {
		pre = "[dry-run] "
	}

	em.Plan(plan)

	efi := steps.DefaultEnv().HasEFI()
	target := steps.ResolveTarget(cfg.Bootloader.Target, efi)
	parts := steps.Layout(cfg.Disk.Device, cfg.Swap(), target)

	for _, s := range plan[:last+1] {
		t0 := time.Now()
		em.StepStart(s.ID, s.ID == "validate")

		if s.ID == "validate" {
			fw := "bios"
			if efi {
				fw = "uefi"
			}
			em.Log("info", s.ID, fmt.Sprintf("configuración válida; firmware %s, arranque %s, escritorio %s", fw, target, cfg.Desktop.Default))
			if p := preflight(em, cfg, s.ID, dry); p != nil {
				em.StepDone(s.ID, false, time.Since(t0))
				exit := events.ExitStepFailed
				if p.Code == events.ErrConfigInvalid {
					exit = events.ExitConfigInvalid
				}
				return fail(exit, s.ID, p.Code, p.Message, "")
			}
			em.StepDone(s.ID, true, time.Since(t0))
			if cancelled.Load() {
				em.Warning(s.ID, "W_CANCELLED", "instalación cancelada por el usuario")
				em.Done(false, false, time.Since(start))
				return events.ExitCancelled
			}
			continue
		}

		cmds, known, berr := buildCmds(s.ID, cfg, parts)
		if berr != nil {
			em.StepDone(s.ID, false, time.Since(t0))
			return fail(events.ExitStepFailed, s.ID, events.ErrInternal, berr.Error(), "")
		}
		if !known {
			em.Log("info", s.ID, pre+"paso aún no implementado")
			em.StepDone(s.ID, true, time.Since(t0))
			continue
		}
		if s.ID == "partition" {
			for _, p := range parts {
				em.Log("info", s.ID, pre+describe(p))
			}
		}
		if s.ID == "unpack" {
			zero := 0.0
			em.StepProgress(s.ID, &zero, "")
		}
		for i, c := range cmds {
			if err := ex.Run(s.ID, c); err != nil {
				em.StepDone(s.ID, false, time.Since(t0))
				code := stepError[s.ID]
				if code == "" {
					code = events.ErrInternal
				}
				detail := ""
				var ce *steps.CmdError
				if errors.As(err, &ce) {
					detail = strings.Join(ce.Tail, "\n")
				}
				return fail(events.ExitStepFailed, s.ID, code, err.Error(), detail)
			}
			if s.ID != "unpack" && len(cmds) > 1 {
				p := float64(i+1) * 100 / float64(len(cmds))
				em.StepProgress(s.ID, &p, "")
			}
		}
		if s.ID == "unpack" {
			full := 100.0
			em.StepProgress(s.ID, &full, "")
		}
		em.StepDone(s.ID, true, time.Since(t0))
	}
	em.Done(true, false, time.Since(start))
	return events.ExitOK
}
