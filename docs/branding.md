# Branding (v1)

La marca del instalador es una carpeta de datos. Cambiarla no requiere
recompilar nada, y la TUI y la GUI la leen igual.

- En el repo: `branding/<nombre>/`
- Instalada: `/usr/share/emancipation-installer/branding/<nombre>/`
- El motor y las interfaces la eligen con `--branding <ruta>`; sin ese
  argumento usan `emancipation`.

## Contenido de la carpeta

```
branding/emancipation/
├── branding.toml      # obligatorio
├── logo.svg           # logo principal (GUI)
├── logo.txt           # logo en caracteres (TUI), opcional
├── wallpaper.png      # fondo de las pantallas, 1920x1080 recomendado
├── desktop/
│   ├── sway.png       # captura de la pantalla "Escritorio", 16:9 (960x540)
│   └── niri.png
└── slides/            # diapositivas de la pantalla de progreso
    ├── 01.png         # 1280x720 recomendado
    └── 02.png
```

## branding.toml

```toml
[identity]
name    = "Emancipation-OS"
version = "0.1.8"
tagline = "Tu sistema, tus reglas"

[palette]
background = "#1d2023"
surface    = "#2f343a"
accent     = "#3daee9"
text       = "#eff0f1"
text_muted = "#a1a9b1"
success    = "#27ae60"
warning    = "#f67400"
error      = "#da4453"

[fonts]
ui   = "Source Sans 3"
mono = "CaskaydiaCove Nerd Font Mono"

[assets]
logo      = "logo.svg"
logo_text = "logo.txt"
wallpaper = "wallpaper.png"

[desktop.sway]
title       = "Sway"
description = "Ventanas en mosaico, ligero y muy configurable."
screenshot  = "desktop/sway.png"

[desktop.niri]
title       = "Niri"
description = "Ventanas en una tira con desplazamiento, moderno y fluido."
screenshot  = "desktop/niri.png"

[[slides]]
image   = "slides/01.png"
title   = "Bienvenido"
text    = "Un sistema construido desde cero, a tu medida."
seconds = 8

[strings.es]
welcome_title = "Bienvenido a Emancipation-OS"
welcome_body  = "Vamos a instalar tu sistema en unos minutos."
desktop_title = "Elige tu escritorio"
desktop_body  = "Los dos quedan instalados; puedes cambiar al otro al iniciar sesión."
summary_title = "Resumen"
progress_title = "Instalando"
finish_title  = "Todo listo"
finish_body   = "Reinicia para empezar a usar tu sistema."

[strings.en]
welcome_title = "Welcome to Emancipation-OS"
```

## Reglas

- Obligatorios: `identity.name` y los cuatro colores `background`, `surface`,
  `accent` y `text`. Lo demás tiene valor por defecto.
- Todo color es `#rrggbb`.
- Toda ruta es relativa a la carpeta del branding. Una ruta absoluta o con
  `..` se rechaza.
- Un recurso opcional que falte no es un error: la interfaz usa un color
  liso (`background`) en lugar del fondo, y el nombre en texto en lugar del
  logo.
- `[strings.<idioma>]` se elige según el idioma de la primera pantalla. Una
  clave ausente en ese idioma se toma de `strings.es`.
- `[[slides]]` puede repetirse; se muestran en orden y en bucle. `seconds`
  es el tiempo por diapositiva (defecto 8). Sin diapositivas, la pantalla de
  progreso muestra solo la barra y el log.
- `fonts` son nombres de familia de fontconfig. Si no existen, la interfaz
  usa `sans-serif` y `monospace` del sistema.

## Cómo la usa cada interfaz

- GUI: pinta con la paleta, dibuja `logo.svg` y `wallpaper.png`, y carga las
  tipografías por fontconfig.
- TUI: usa los colores en 24 bits y baja a 256 colores si la terminal no los
  soporta. Muestra `logo.txt` si existe, o el nombre en texto si no. Ignora
  imágenes, diapositivas y capturas.
- El motor no usa el branding salvo para el título del log.

## Ampliaciones (3 nov 2026)

- Los nombres de archivo son libres: manda el manifiesto. El árbol de arriba
  solo muestra los nombres por defecto.
- Formatos: el logo y las capturas pueden ser SVG, PNG o JPG; los fondos y las
  diapositivas, PNG o JPG. La TUI ignora todas las imágenes.
- Fondo por escritorio: `[desktop.<id>] wallpaper` sustituye a
  `[assets] wallpaper`. En la pantalla "Elige tu escritorio" la GUI cambia el
  fondo al elegir. Los fondos deben admitir una capa oscura semitransparente
  encima, porque el contenido se dibuja sobre ellos.
- Capturas de escritorio: cualquier proporción; la GUI las ajusta dentro de la
  tarjeta sin recortarlas.
- `palette.stripe`: lista opcional de colores para la franja superior, como la
  de las diapositivas. `palette.border` es el color de los bordes de tarjeta.
- Diapositivas: la imagen ya trae su texto. `title` y `text` son su equivalente
  en texto: la TUI los muestra en rotación y la GUI no los dibuja encima.
