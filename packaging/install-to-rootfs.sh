#!/bin/sh
# Instala el motor, el branding y los ejemplos dentro de un rootfs de
# Emancipation-OS. Uso (con sudo): sh packaging/install-to-rootfs.sh /mnt/lfs/Emancipation-OS
set -eu
ROOT=${1:?uso: $0 <ruta-del-rootfs>}
REPO=$(cd "$(dirname "$0")/.." && pwd)
BIN="$REPO/bin/emancipation-install-static"
[ -x "$BIN" ] || { echo "falta $BIN: compila con CGO_ENABLED=0 go build -o bin/emancipation-install-static ./cmd/emancipation-install" >&2; exit 1; }
[ -d "$ROOT/usr/bin" ] || { echo "$ROOT no parece un rootfs" >&2; exit 1; }
SHARE="$ROOT/usr/share/emancipation-installer"
install -Dm755 "$BIN" "$ROOT/usr/bin/emancipation-install"
install -Dm755 "$REPO/packaging/ver-eventos.sh" "$ROOT/usr/bin/ver-eventos"
rm -rf "$SHARE"
install -d -m755 "$SHARE"
cp -r "$REPO/branding" "$SHARE/branding"
cp -r "$REPO/examples" "$SHARE/examples"
chown -R root:root "$SHARE"
echo "instalado en $ROOT:"
ls -l "$ROOT/usr/bin/emancipation-install" "$ROOT/usr/bin/ver-eventos"
du -sh "$SHARE"
