package steps

import (
	"strconv"
	"strings"
)

// UnpackCmds extrae el sistema de origen (un squashfs) en target. -f permite
// escribir sobre los directorios que ya existen (los puntos de montaje) y
// -percentage hace que unsquashfs imprima solo el porcentaje, un número por línea.
func UnpackCmds(squashfs, target string) []Cmd {
	return []Cmd{cmd("unsquashfs", "-f", "-percentage", "-d", target, squashfs)}
}

// ParsePercent interpreta una línea de progreso ("42", "42%" o "42.5").
// Devuelve false para cualquier otra línea.
func ParsePercent(line string) (float64, bool) {
	s := strings.TrimSuffix(strings.TrimSpace(line), "%")
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' {
			return 0, false
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 || v > 100 {
		return 0, false
	}
	return v, true
}
