# mkdocx — diseño

## Objetivo
CLI en Go, liviano y universal, que genera `.docx` a partir de Markdown y rellena plantillas `.docx` con datos JSON. Incluye un skill (`SKILL.md`) para que agentes lo usen sin fricción.

Éxito: `mkdocx build in.md -o out.docx` produce un docx válido que abre en Word/LibreOffice; binario estático de un solo archivo; un agente lo usa leyendo solo `SKILL.md`.

## Decisiones (aprobadas)
- Librería docx existente: **docxgo v2.14.0** (`github.com/mmonterroca/docxgo/v2`, MIT, sin dependencias transitivas) + **goldmark** (Markdown) .
- Dos flujos: `build` (Markdown→docx, núcleo) y `fill` (plantilla+JSON).
- Fuera de v1: notas al pie, HTML embebido, fórmulas, bucles en plantillas (docxgo solo soporta `{{clave}}`/`{{a.b}}`).

## CLI
```
mkdocx build <entrada.md|-> -o <salida.docx|-> [--title T] [--author A]
mkdocx fill  <plantilla.docx> --data <datos.json|-> -o <salida.docx> [--strict]
mkdocx --version | --help
```
- `-` = stdin/stdout. En éxito con `-o` archivo, imprime solo la ruta de salida en stdout.
- Errores: una línea en stderr, código de salida ≠ 0 (1 = uso/entrada, 2 = fallo de generación).
- Flags con stdlib `flag`; sin cobra.

## Estructura
```
cmd/mkdocx/main.go     # flags y despacho
internal/md/           # goldmark AST → modelo de bloques
internal/render/       # modelo de bloques → docxgo (único paquete que importa docxgo)
internal/fill/         # MergeTemplate + JSON
skill/SKILL.md
```
Modelo intermedio: Heading, Paragraph (runs con negrita/cursiva/código/enlace), List (anidada, ordenada o no), Table, CodeBlock, Quote, Image, PageBreak.

## Cobertura Markdown v1
Títulos H1–H6, párrafos, **negrita**, *cursiva*, `código`, listas anidadas, bloques de código, citas, tablas GFM, imágenes locales, enlaces, salto de página (`<!-- pagebreak -->`).

## Pruebas
- Golden: md → docx → extraer `word/document.xml` y comparar elementos clave.
- Validez: unzip + parseo XML de todas las partes; LibreOffice headless opcional si existe.
- `fill`: plantilla mínima + JSON; modo estricto falla con clave faltante.
- Tamaño de binario medido; si > 10 MB se reabre la decisión de dependencias.

## Entorno
Go 1.27.2 instalado en `~/.local/go` (agregar `~/.local/go/bin` al PATH).
