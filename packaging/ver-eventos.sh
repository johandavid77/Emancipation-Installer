#!/bin/sh
# Muestra los eventos del motor en texto legible (sin jq), en vivo.
# Uso: emancipation-install --config c.json --dry-run | ver-eventos
sed -u -n \
  -e '/"level":"debug"/d' \
  -e 's/.*"type":"step_start","step":"\([a-z]*\)".*/== \1/p' \
  -e 's/.*"type":"step_progress","step":"\([a-z]*\)","percent":\([0-9.]*\).*/   \1 \2%/p' \
  -e 's/.*"type":"\(log\|warning\|error\)".*"message":"\([^"]*\)".*/\1: \2/p' \
  -e 's/.*"type":"done","success":\([a-z]*\).*/FIN success=\1/p'
