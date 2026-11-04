#!/bin/sh
# Regenera el squashfs y la ISO de Emancipation-OS desde el rootfs.
# Uso (con sudo): sh packaging/build-iso.sh <versión> [ruta-del-rootfs]
set -eu
VER=${1:?uso: $0 <versión> [rootfs]}
ROOT=${2:-/mnt/lfs/Emancipation-OS}
[ "$(id -u)" -eq 0 ] || { echo "ejecútalo con sudo" >&2; exit 1; }
USER_HOME=$(getent passwd "${SUDO_USER:-root}" | cut -d: -f6)
BUILD="$USER_HOME/emancipation-build"
SQ="$BUILD/iso-root/LiveOS/emancipation-rootfs.squashfs"
ISO="$BUILD/emancipation-os-$VER.iso"

for d in dev proc sys run; do umount -R "$ROOT/$d" 2>/dev/null || true; done
if findmnt -R -n "$ROOT" | grep -qE "$ROOT/(dev|proc|sys|run)"; then
  echo "ATENCIÓN: quedan montajes de dev/proc/sys/run dentro del rootfs; no sigo" >&2
  exit 1
fi
[ -f "$ROOT/usr/bin/emancipation-install" ] || echo "AVISO: el rootfs no tiene emancipation-install (corre packaging/install-to-rootfs.sh)" >&2

echo "== squashfs"
mksquashfs "$ROOT" "$SQ" -noappend -comp xz -e sources root/.rustup root/.cargo opt/go -progress

echo "== comprobando el contenido"
LIST=$(mktemp)
unsquashfs -l "$SQ" > "$LIST"
for f in usr/bin/emancipation-install usr/bin/ver-eventos usr/share/emancipation-installer/branding/emancipation/branding.toml; do
  grep -q "squashfs-root/${f}\$" "$LIST" || { echo "FALTA en el squashfs: $f" >&2; rm -f "$LIST"; exit 1; }
done
if grep -qE 'squashfs-root/(sources|root/\.rustup|root/\.cargo|opt/go)/' "$LIST"; then
  echo "Hay rutas excluidas dentro del squashfs" >&2; rm -f "$LIST"; exit 1
fi
rm -f "$LIST"

echo "== ISO"
grub-mkrescue -o "$ISO" "$BUILD/iso-root" -iso-level 3 -- -volid EMANCIPATION_LIVE
ls -lh "$ISO"
sha256sum "$ISO"
