# Eventos de progreso (v1)

El motor escribe un objeto JSON por línea (JSON Lines) en `stdout`.
Las interfaces (TUI y GUI) solo leen y pintan: nunca instalan nada.
`stderr` queda para fallos del propio motor y no es parte del contrato.

Todo evento lleva estos campos:

| Campo | Descripción |
|---|---|
| `v` | Versión del protocolo. Hoy `1`. |
| `ts` | Marca de tiempo RFC 3339 con zona. |
| `type` | Tipo de evento (ver abajo). |

## Entrada del motor

- `emancipation-install --config <archivo.json>` (o `-` para leerla por stdin).
- `--dry-run` fuerza `options.dry_run`. Cualquier señal `SIGTERM` pide cancelar.
- La cancelación solo se acepta mientras `cancel_allowed` sea `true`
  (ver `step_start`); después se ignora y se termina el paso en curso.

## Tipos de evento

### `hello`
Primer evento. Confirma versión y modo.
```json
{"v":1,"ts":"2026-11-03T10:00:00-05:00","type":"hello","engine":"0.1.0","dry_run":true,"log_path":"/var/log/emancipation-install.log"}
```

### `plan`
Lista ordenada de pasos con su peso. La suma de pesos es 100.
Las interfaces calculan el avance total con esto.
```json
{"v":1,"ts":"...","type":"plan","steps":[
  {"id":"validate","label":"Comprobando la configuración","weight":1},
  {"id":"partition","label":"Particionando el disco","weight":3},
  {"id":"format","label":"Creando sistemas de archivos","weight":3},
  {"id":"mount","label":"Montando el destino","weight":1},
  {"id":"unpack","label":"Copiando el sistema","weight":70},
  {"id":"fstab","label":"Generando fstab","weight":1},
  {"id":"system","label":"Configurando idioma, teclado y hora","weight":4},
  {"id":"user","label":"Creando el usuario","weight":2},
  {"id":"desktop","label":"Configurando el escritorio","weight":2},
  {"id":"bootloader","label":"Instalando el cargador de arranque","weight":8},
  {"id":"services","label":"Activando servicios","weight":2},
  {"id":"finish","label":"Desmontando y terminando","weight":3}
]}
```
Los `id` son estables y son lo que las interfaces deben usar. Los `label`
son un texto por defecto en español, que la interfaz puede reemplazar.

### `step_start`
```json
{"v":1,"ts":"...","type":"step_start","step":"partition","cancel_allowed":false}
```
`cancel_allowed` es `true` solo en `validate`. Desde `partition` ya se
modifica el disco y la interfaz debe ocultar el botón de cancelar.

### `step_progress`
```json
{"v":1,"ts":"...","type":"step_progress","step":"unpack","percent":42.5,"detail":"usr/lib/libxul.so"}
```
`percent` va de 0 a 100, o es `null` si el paso no puede medirse.
`detail` es opcional y solo informativo. Se emite como máximo 4 veces por segundo.

### `step_done`
```json
{"v":1,"ts":"...","type":"step_done","step":"partition","ok":true,"elapsed_ms":1840}
```

### `log`
Línea de log para mostrar en vivo. Es la misma que se escribe en `log_path`.
```json
{"v":1,"ts":"...","type":"log","level":"info","step":"format","message":"mkfs.fat -F32 /dev/sda1"}
```
`level` es `debug`, `info`, `warn` o `error`. La contraseña nunca aparece.

### `warning`
Algo no bloqueante que la interfaz puede destacar.
```json
{"v":1,"ts":"...","type":"warning","step":"bootloader","code":"W_EFIVARS","message":"efivarfs no estaba montado; se montó automáticamente"}
```

### `error`
```json
{"v":1,"ts":"...","type":"error","step":"mount","code":"E_MOUNT","fatal":true,"message":"No se pudo montar /dev/sda3","detail":"mount: wrong fs type ...","log_path":"/var/log/emancipation-install.log"}
```
Tras un error `fatal` el motor emite `done` con `success:false` y termina.

### `done`
Último evento, siempre.
```json
{"v":1,"ts":"...","type":"done","success":true,"reboot_required":true,"elapsed_ms":412000}
```

## Códigos de error

| Código | Significado |
|---|---|
| `E_CONFIG_INVALID` | La configuración no pasó la validación (ver `docs/config.md`). |
| `E_DISK_NOT_FOUND` | El disco indicado no existe. |
| `E_DISK_BUSY` | El disco está montado o es el medio de arranque. |
| `E_PARTITION` | Falló la creación de la tabla o las particiones. |
| `E_FORMAT` | Falló un `mkfs`. |
| `E_MOUNT` | Falló un montaje. |
| `E_UNPACK` | Falló la copia desde el squashfs. |
| `E_SYSTEM` | Falló la configuración de idioma, teclado, hora o hostname. |
| `E_USER` | Falló la creación del usuario. |
| `E_BOOTLOADER` | Falló la instalación de GRUB. |
| `E_INTERNAL` | Error del propio motor. |

## Código de salida del proceso

| Código | Significado |
|---|---|
| `0` | Instalación correcta (o dry-run correcto). |
| `1` | Falló un paso de la instalación. |
| `2` | Configuración inválida. |
| `130` | Cancelada por el usuario. |

## Log persistente

El motor escribe cada línea de log en `log_path` y hace `fsync` en cada una,
de modo que un cuelgue o un reinicio no pierda el final. Como en el live
`/var/log` vive en RAM, al fallar la interfaz debe ofrecer copiar ese archivo
a otro medio. El motor imprime la ruta exacta en el evento `error`.
