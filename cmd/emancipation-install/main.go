// Command emancipation-install es el motor de instalación: lee la
// configuración, ejecuta los pasos y emite eventos JSON Lines por stdout
// (docs/events.md). Por ahora solo funciona en modo --dry-run.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
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

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func efiPresent() bool {
	_, err := os.Stat("/sys/firmware/efi")
	return err == nil
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

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	start := time.Now()
	fs := flag.NewFlagSet("emancipation-install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", "", "archivo de configuración JSON ('-' para stdin)")
	dryFlag := fs.Bool("dry-run", false, "describir lo que se haría sin tocar nada")
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
		em.Hello(engineVersion, *dryFlag, config.DefaultLogPath)
		em.Error("validate", events.ErrConfigInvalid, true, err.Error(), "", "")
		em.Done(false, false, time.Since(start))
		return events.ExitConfigInvalid
	}
	dry := *dryFlag || cfg.Options.DryRun

	var sink events.LogSink
	lf, lerr := logging.Open(cfg.Options.LogPath)
	if lerr == nil {
		sink = lf
		defer lf.Close()
	}
	em := events.New(stdout, sink)
	em.AddSecret(cfg.User.Password)
	em.Hello(engineVersion, dry, cfg.Options.LogPath)

	fail := func(step, code, msg, detail string) int {
		em.Error(step, code, true, msg, detail, cfg.Options.LogPath)
		em.Done(false, false, time.Since(start))
		return events.ExitStepFailed
	}
	if lerr != nil {
		if !dry {
			return fail("validate", events.ErrInternal, "no se pudo abrir el log de la instalación", lerr.Error())
		}
		em.Warning("validate", "W_LOG", "no se pudo abrir el log ("+lerr.Error()+"); el dry-run continúa sin archivo")
	}
	if !dry {
		return fail("validate", events.ErrInternal, "la instalación real aún no está implementada; use --dry-run", "")
	}

	plan := events.DefaultPlan()
	em.Plan(plan)

	efi := efiPresent()
	target := steps.ResolveTarget(cfg.Bootloader.Target, efi)
	parts := steps.Layout(cfg.Disk.Device, cfg.Swap(), target)

	type build func() ([]steps.Cmd, error)
	builders := map[string]build{
		"partition": func() ([]steps.Cmd, error) { return steps.PartitionCmds(cfg.Disk.Device, parts), nil },
		"format":    func() ([]steps.Cmd, error) { return steps.FormatCmds(parts), nil },
		"mount":     func() ([]steps.Cmd, error) { return steps.MountCmds(parts, targetDir) },
	}

	for _, s := range plan {
		t0 := time.Now()
		em.StepStart(s.ID, s.ID == "validate")
		switch s.ID {
		case "validate":
			fw := "bios"
			if efi {
				fw = "uefi"
			}
			em.Log("info", s.ID, fmt.Sprintf("configuración válida; firmware %s, arranque %s, escritorio %s", fw, target, cfg.Desktop.Default))
			preflightWarn(em, cfg, s.ID)
		case "partition":
			for _, p := range parts {
				em.Log("info", s.ID, "[dry-run] "+describe(p))
			}
		}
		if b := builders[s.ID]; b != nil {
			cmds, berr := b()
			if berr != nil {
				em.StepDone(s.ID, false, time.Since(t0))
				return fail(s.ID, events.ErrInternal, berr.Error(), "")
			}
			for _, c := range cmds {
				em.Log("info", s.ID, "[dry-run] "+c.String())
			}
		} else if s.ID != "validate" {
			em.Log("info", s.ID, "[dry-run] paso aún no implementado")
		}
		em.StepDone(s.ID, true, time.Since(t0))
	}
	em.Done(true, false, time.Since(start))
	return events.ExitOK
}
