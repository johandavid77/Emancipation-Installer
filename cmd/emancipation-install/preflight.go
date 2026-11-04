package main

import (
	"strings"

	"github.com/johandavid77/Emancipation-Installer/internal/config"
	"github.com/johandavid77/Emancipation-Installer/internal/events"
	"github.com/johandavid77/Emancipation-Installer/internal/steps"
)

// preflight ejecuta las comprobaciones contra el sistema. En dry-run los
// problemas son avisos; en la instalación real devuelve el primero para que el
// motor lo reporte como error fatal (todos quedan en el log).
func preflight(em *events.Emitter, cfg *config.Config, step string, dry bool) *steps.Problem {
	problems := steps.Preflight(cfg, steps.DefaultEnv())
	if len(problems) == 0 {
		em.Log("info", step, "comprobaciones del sistema correctas")
		return nil
	}
	if dry {
		for _, p := range problems {
			code := "W_" + strings.TrimPrefix(p.Code, "E_")
			em.Warning(step, code, p.Message+" (en una instalación real esto detendría el proceso)")
		}
		return nil
	}
	for _, p := range problems {
		em.Log("error", step, p.Code+": "+p.Message)
	}
	return &problems[0]
}
