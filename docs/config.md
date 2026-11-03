# Configuración de instalación (v1)

Es lo único que las interfaces (TUI y GUI) le entregan al motor. JSON, UTF-8.
El motor la valida entera antes de tocar ningún disco y nunca escribe la
contraseña en el log.

## Ejemplo

```json
{
  "version": 1,
  "locale": { "language": "es_ES.UTF-8", "timezone": "America/Bogota", "keymap": "es" },
  "disk": { "device": "/dev/sda", "confirm_erase": "/dev/sda", "swap_mib": 2048 },
  "bootloader": { "target": "auto" },
  "hostname": "emancipation-pc",
  "user": { "name": "usuario", "full_name": "Usuario", "password": "secreto", "admin": true, "autologin": false },
  "desktop": { "default": "sway" },
  "options": { "dry_run": true, "log_path": "/var/log/emancipation-install.log" }
}
```

## Campos

| Campo | Tipo | Obligatorio | Descripción |
|---|---|---|---|
| `version` | int | sí | Versión del formato. Hoy `1`. |
| `locale.language` | string | sí | Locale del sistema (`es_ES.UTF-8`). Debe existir en el locale-archive. |
| `locale.timezone` | string | sí | Zona IANA (`America/Bogota`). |
| `locale.keymap` | string | sí | Teclado de consola y de sway/niri (`es`). |
| `disk.device` | string | sí | Disco entero a instalar (`/dev/sda`), nunca una partición. |
| `disk.confirm_erase` | string | sí | Debe ser idéntico a `disk.device`. Confirma que se borra todo. |
| `disk.swap_mib` | int | no | Tamaño del swap en MiB. `0` lo desactiva. Defecto: `2048`. |
| `bootloader.target` | string | no | `auto`, `uefi`, `bios` o `hybrid`. Defecto: `auto`. |
| `hostname` | string | sí | Nombre de equipo (RFC 1123). |
| `user.name` | string | sí | Usuario nuevo: `^[a-z_][a-z0-9_-]{0,31}$`. |
| `user.full_name` | string | no | Nombre para mostrar. |
| `user.password` | string | sí | Texto plano; el motor la cifra dentro del sistema instalado. |
| `user.admin` | bool | no | Lo agrega al grupo `wheel` (doas). Defecto: `true`. |
| `user.autologin` | bool | no | Ly entra solo con este usuario. Defecto: `false`. |
| `desktop.default` | string | sí | `sway` o `niri`. Los dos quedan instalados; este es el predeterminado. |
| `source.squashfs` | string | no | Defecto: `/run/miso/bootmnt/LiveOS/emancipation-rootfs.squashfs`. |
| `options.dry_run` | bool | no | Si es `true`, el motor describe lo que haría y no toca nada. |
| `options.log_path` | string | no | Log en vivo. Defecto: `/var/log/emancipation-install.log`. |

## Disposición del disco (v1)

Siempre disco completo, tabla GPT:

1. `bios_grub` de 1 MiB, solo si `bootloader.target` es `bios` o `hybrid`.
2. ESP de 512 MiB en FAT32, montada en `/boot/efi`.
3. Swap de `disk.swap_mib` MiB, si es mayor que 0.
4. Raíz btrfs con el resto del disco y subvolúmenes `@` (`/`), `@home`
   (`/home`) y `@snapshots` (`/.snapshots`).

Fuera de v1: LUKS, dual-boot, particionado manual, otros sistemas de archivos.

## Reglas de validación

- `disk.device` existe, es un disco entero, no está montado y no es el medio
  de arranque (el que tiene la etiqueta `EMANCIPATION_LIVE`).
- `disk.confirm_erase` es exactamente igual a `disk.device`.
- `bootloader.target` `uefi` exige que el firmware sea UEFI. `auto` lo detecta
  con `/sys/firmware/efi`.
- `desktop.default` es `sway` o `niri` y su archivo `.desktop` existe en el
  sistema de origen.
- `locale.language` y `locale.keymap` existen en el sistema de origen.
- `user.password` no está vacía y `user.name` no es `root` ni el usuario live.

Un error de validación aborta antes de cualquier paso destructivo y se emite
como evento `error` (ver `docs/events.md`).
