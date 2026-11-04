package steps

import "strings"

// logStdin registra la entrada estándar de un comando, línea a línea, salvo que
// sea sensible (QuietStdin).
func logStdin(l Logger, step string, c Cmd, prefix string) {
	if c.Stdin == "" || c.QuietStdin {
		return
	}
	for _, line := range strings.Split(strings.TrimRight(c.Stdin, "\n"), "\n") {
		l.Log("info", step, prefix+line)
	}
}
