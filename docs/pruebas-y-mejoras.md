# mkdocx — pruebas y plan de mejoras

Fecha: 2026-10-11 · Versión probada: `c9cd3b0` (instalada con `go install …@latest` → `v0.0.0-20261011033754-c9cd3b006247`).

## Qué se probó

| Prueba | Resultado |
|---|---|
| `go install github.com/bioodev/mkdocx/cmd/mkdocx@latest` | OK; `--version` muestra la pseudo-versión del módulo |
| 25 archivos `.md` reales del disco (READMEs, specs, skills) | 23 OK; 2 fallan por imágenes (E1) |
| Abrir los 23 `.docx` en LibreOffice (conversión a PDF) | 23 / 23 se abren |
| Batería de casos límite de Markdown (20 casos) | Ver E2–E5 |
| `fill` con una plantilla hecha en LibreOffice (cabecera, pie, tabla, marcador con formato mixto) | Cabecera, pie y tabla OK; ver E6 |
| Casos límite del CLI | Ver E7–E10 |
| Rendimiento con 2,4 MB de Markdown (20.000 secciones) | 3 s, **827 MB de RAM** (E11) |

Todo esto funciona bien: escapes y entidades, autoenlaces, listas anidadas hasta el nivel 11 (se limita al 9), CRLF + front matter, emojis, RTL, caracteres de ancho cero, tablas con formato y `\|`, citas anidadas, entrada vacía, `/dev/null`, stdin/stdout y escapado XML de los valores en `fill` (incluido `\n` → salto de línea).

## Errores encontrados

Prioridad: **A** = pierde contenido o falla en silencio · **B** = mala experiencia de uso · **C** = cosmético.

| # | Prio | Error | Cómo reproducirlo |
|---|---|---|---|
| E6 | **A** | En `fill`, un marcador partido en runs **con formato distinto** (por ejemplo, `{{to` en negrita y `tal}}` normal, algo habitual al editar en Word) **no se reemplaza, no se avisa y `--strict` tampoco lo detecta**. | Plantilla con `<w:r><w:rPr><w:b/></w:rPr><w:t>{{to</w:t></w:r><w:r><w:t>tal}}</w:t></w:r>` + `{"total":1}` → queda `{{total}}` en la salida y el exit es 0 |
| E7 | **A** | `-o` igual a la entrada sobrescribe el `.md` de origen sin preguntar. | `mkdocx build a.md -o a.md` → `a.md` pasa a ser un zip |
| E1 | **A** | Una sola imagen SVG/GIF/WebP o una imagen local que no existe **aborta todo el documento** (exit 1). Es habitual en READMEs (logos `.svg`, badges). | `![](logo.svg)` o `![](nada.png)` |
| E2 | B | Con un BOM UTF-8 al inicio, el primer `# Título` sale como texto literal. | `printf '\xef\xbb\xbf# T' \| mkdocx build - -o -` |
| E3 | B | Las listas de tareas GFM salen como `[ ]` / `[x]` literales. | `- [ ] pendiente` |
| E4 | B | Las notas al pie salen literales (`[^1]`) y sin aviso (está documentado como límite). | `Texto[^1].\n\n[^1]: nota` |
| E5 | C | `0. cero` empieza en 1 (`Start = 0` se trata como «sin inicio»). | `0. a\n1. b` |
| E8 | B | `-o <directorio>` da exit 2 con un mensaje críptico (`rename … file exists`) en vez de exit 1 con un mensaje claro. | `mkdocx build a.md -o .` |
| E9 | B | `mkdocx build --help` / `fill --help` → exit 1 «flag: help requested»; no hay ayuda por subcomando. | — |
| E10 | C | Una plantilla que no es `.docx` muestra la cadena de error interna de docxgo (`operation=… code=INTERNAL_ERROR …`). Un JSON con basura al final (`{"a":1}{"b":2}`) se acepta en silencio. | `mkdocx fill a.md …` |
| E11 | B | Uso de memoria de ~350× el tamaño de entrada (2,4 MB → 827 MB; 600 KB → 211 MB). | `big.md` de 20.000 secciones |
| E12 | C | El ancho máximo de imagen (624 px = 6,5") supera el área útil de la página A4 que genera docxgo (11906 twips − 2×1440 = 6,27" ≈ 602 px): las imágenes anchas se salen un poco del margen. | `![](ancha.png)` con ≥ 624 px |
| E13 | C | Las reglas `---` y los comentarios HTML normales generan avisos en casi todos los READMEs (ruido). Las citas anidadas (`>>`) no se distinguen visualmente. | — |

Arrastrados de la revisión anterior: el texto alternativo de las imágenes no resuelve escapes; los párrafos y el código dentro de un ítem de lista salen sin sangría; `walkRaw` duplica casi todo `walk`.

## Plan de mejoras

### Fase 1 — Correcciones (prioridad A)
1. **E6 · Marcadores con formato mixto.** Antes de `MergeTemplate`, unir el texto de los runs de cada párrafo que contiene `{{…}}` partido: concatenar el texto en el primer run del marcador (con su formato) y vaciar los demás. Después del merge, buscar en todas las partes (`document`, `header*`, `footer*`) los `{{clave}}` que sigan presentes y cuya clave esté en los datos, y avisar de cada uno; con `--strict`, tratarlos como error. Test con la plantilla de LibreOffice de esta prueba (guardarla en `testdata/`).
2. **E7 · Proteger la entrada.** Si `-o` resuelve al mismo archivo que la entrada (`os.SameFile`), devolver exit 1. En `fill` se permite sobrescribir la plantilla solo con `--force` (o también prohibirlo).
3. **E1 · Imágenes que no se pueden incluir.** Por defecto, aviso + texto alternativo como párrafo (igual que las remotas). Con `--strict` en `build`, que sea error. Valorar soporte de GIF (`image/gif` de la stdlib → convertir a PNG en memoria); SVG queda fuera (requiere rasterizar).

### Fase 2 — Calidad de uso (prioridad B)
4. **E2:** quitar el BOM en `md.Parse` antes del front matter.
5. **E3:** listas de tareas: activar `extension.TaskList` y emitir `☐` / `☑` al inicio del ítem.
6. **E4:** notas al pie: activar `extension.Footnote` y, si docxgo no tiene notas al pie nativas, renderizarlas como superíndice `[n]` más una sección «Notas» al final; como mínimo, avisar.
7. **E8 / E9 / E10:** validar que `-o` no sea un directorio, ayuda por subcomando (`fs.Usage` y exit 0 con `-h`), resumir los errores de docxgo (`no es un .docx válido`) y rechazar el JSON con datos extra (`dec.More()`).
8. **E11:** perfilar con `pprof`. Sospechosos: `docxgo` mantiene el árbol completo más un buffer intermedio, y `injectNumbering` vuelve a comprimir todo el zip en memoria. Medir antes de cambiar; el objetivo es menos de 50× la entrada.

### Fase 3 — Pulido (prioridad C)
9. **E12:** calcular el ancho máximo a partir del tamaño de página real (602 px para A4 con márgenes de 1").
10. **E5:** distinguir «sin inicio» de `0` (`Start` como `*int` o una bandera aparte).
11. **E13:** agrupar los avisos repetidos con contador (`regla horizontal ignorada (×7)`), no avisar de comentarios HTML y añadir sangría por nivel a las citas anidadas y a los bloques dentro de ítems.
12. Deuda técnica: unificar `walk` y `walkRaw` con un parámetro `raw`; resolver escapes en el texto alternativo.

### Fase 4 — Funcionalidad nueva (a valorar)
- `fill` con arrays (filas de tabla repetidas) si docxgo lo permite o con un preprocesado propio.
- `{{ clave }}` con espacios (normalizar antes del merge).
- `--reference-doc estilos.docx` para tomar estilos y página de una plantilla (como pandoc).
- Alineación de columnas en tablas (`:--:`).
- ~~Release etiquetado con binarios en GitHub~~ → hecho: `make publish VERSION=vX.Y.Z` (`scripts/release.sh`, sin gh) + `skills/mkdocx/scripts/install.sh`. Falta publicar el primer release (`v0.1.0`).

## Reproducir las pruebas
Los scripts de esta sesión no se guardaron en el repo. Para que esto sea repetible: añadir a `testdata/` la plantilla de LibreOffice (`tpl.fodt` → `tpl.docx`) y un `torture.md` con los casos E2–E5, y un `make smoke` que convierta `testdata/*.md` y, si `soffice` existe, valide con `soffice --headless --convert-to pdf`.
