package main

import (
	"strings"

	"github.com/johandavid77/Emancipation-Installer/internal/config"
	"github.com/johandavid77/Emancipation-Installer/internal/events"
	"github.com/johandavid77/Emancipation-Installer/internal/steps"
)

// preflightWarn ejecuta las comprobaciones contra el sistema. En dry-run los
// problemas se reportan como avisos, porque la máquina de pruebas no es el
// medio live; en la instalación real serán errores fatales.
func preflightWarn(em *events.Emitter, cfg *config.Config, step string) {
	problems := steps.Preflight(cfg, steps.DefaultEnv())
	if len(problems) == 0 {
		em.Log("info", step, "comprobaciones del sistema correctas")
		return
	}
	for _, p := range problems {
		code := "W_" + strings.TrimPrefix(p.Code, "E_")
		em.Warning(step, code, p.Message+" (en una instalación real esto detendría el proceso)")
	}
}
