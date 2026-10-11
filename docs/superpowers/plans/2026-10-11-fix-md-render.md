# Plan A — Correcciones en `internal/md` e `internal/render`

Rama: `fix/md-render` · Worktree: `../mkdocx-wt-md`

## Reglas
- **Solo puedes tocar:** `internal/md/*`, `internal/render/*`, `cmd/mkdocx/args.go`, `cmd/mkdocx/build.go`.
  Otro agente trabaja en paralelo sobre `cmd/mkdocx/main.go`, `cmd/mkdocx/main_test.go`, `internal/fill/*`, `go.mod`, `go.sum`. No los toques. Tampoco `README.md` ni `skill/SKILL.md`.
- Go: `export PATH=$HOME/.local/go/bin:$PATH` (Go 1.27.2).
- TDD: para cada punto, primero escribe el test que falla, comprueba que falla y después corrige.
- Respeta el estilo del código: comentarios y mensajes en español, funciones cortas y sin dependencias nuevas.
- Al terminar cada punto, ejecuta `go vet ./... && go test ./...` y haz un commit en la rama con un mensaje convencional (`fix(md): ...`).
- No hagas push.

## Tareas (en este orden)

### 1. Escapes y entidades (`parse.go`, caso `*ast.Text`)
Ahora `\*no\* AT&amp;T &copy;` sale literal. El texto tiene que pasar por
`util.UnescapePunctuations` y `util.ResolveNumericReferences`/`util.ResolveEntityNames`
(`github.com/yuin/goldmark/util`), igual que hace el renderer HTML de goldmark.
**No** apliques esto dentro de `CodeSpan` ni de los bloques de código.
Test: `\*no\* AT&amp;T &copy;` debe producir el texto `*no* AT&T ©`.

### 2. Autoenlaces (`*ast.AutoLink`)
Ahora `<https://go.dev>` produce `Inlines: nil`. Añade un caso que emita
`Inline{Text: label, URL: url}` con `v.Label(src)` y `v.URL(src)`; para los enlaces de correo, la URL debe empezar por `mailto:`.

### 3. Bloques dentro de ítems de lista y citas (sin pérdida silenciosa)
Ahora `- a\n\n  ```\n  code\n  ```` pierde el código y `> - x\n> - y` produce `"xy"`.
- Cambia el modelo para que un `ListItem` pueda contener bloques arbitrarios **en orden**:
  el texto inicial se queda en `Inlines` y, después, `Blocks []Block`, donde caben `CodeBlock`, `Table`, `Quote`, `List` anidada, párrafos adicionales, etc. Elimina `Children` si queda redundante y actualiza los tests existentes.
- `Quote` pasa a `Quote{Blocks []Block}`. El renderer aplica el estilo `Quote` a los párrafos de la cita y renderiza el resto de bloques en orden.
- Los ítems con varios párrafos (listas "loose") deben conservar los párrafos separados, no unirlos con un espacio.
- Renderer: las listas anidadas siguen usando el nivel +1. Los bloques de código y las tablas dentro de un ítem se renderizan justo después del párrafo del ítem. La sangría es opcional.
- Si algún tipo de bloque no se puede representar, emite un **aviso** en vez de perderlo.
Tests: parser (modelo) y render (el texto de `code` aparece en `document.xml`; `x` e `y` salen en párrafos distintos).

### 4. Front matter (`stripFrontMatter`)
Ahora `---\n\nIntro\n\n---\n\nmore` se come `Intro`. Aplica la regla de pandoc: solo cuenta como front matter si la línea siguiente al `---` inicial **no está en blanco**, y el cierre tiene que ser una línea que contenga exactamente `---` o `...`, opcionalmente seguida de `\r`.
Tests: el caso anterior conserva `Intro` (con aviso de regla horizontal); `---\ntitle: x\n---\n# T` sigue funcionando; también con `\r\n`; y `---\na: b\n----x` **no** es front matter.

### 5. Número inicial de listas ordenadas
Ahora `3. a` empieza en 1. Añade `Start int` a `List` (con `l.Start` de goldmark). En `numbering.go`, cada instancia `<w:num>` tiene que usar `startOverride = Start` en el nivel en que se usa (por defecto 1). Cambia `numberingXML(ordered int)` por la estructura que necesites, por ejemplo `[]int` con los inicios.

### 6. Salto suave dentro de enlaces o código
En `walk`, el `Inline{Text: " "}` del salto suave pierde `URL` y `Code`, así que un enlace que ocupa dos líneas queda partido. Usa una copia de `st` con `Text = " "`.
Test: `[foo\nbar](u)` produce **un único** inline `{Text:"foo bar", URL:"u"}`.

### 7. Imágenes
- **Remotas** (`http://` o `https://`): en el parser, en vez de un bloque `Image`, emite un párrafo con el texto alternativo (o la URL si no hay texto alternativo) enlazado a la URL, más el aviso `imagen remota no soportada: se usa un enlace`.
- **`%20` y otros escapes de ruta:** en el renderer, si la ruta tal cual no existe, prueba con `url.PathUnescape`.
- **Alto máximo:** además de `maxImageWidthPx = 624`, limita el alto a unos 864 px (9" a 96 dpi) **manteniendo la proporción**.
- **Error tipado:** en `render`, define `type InputError struct{ Err error }` (con `Error` y `Unwrap`) y devuélvelo para los errores de imagen (no encontrada o formato no soportado). En `cmd/mkdocx/args.go`, `asUserIfInput` debe usar `errors.As` con `*render.InputError` en lugar de mirar el prefijo del mensaje. El test existente `TestBuildErrores/imagen faltante` tiene que seguir pasando (exit 1).

### 8. Bloques de código: espaciado
Cada línea de un bloque de código es un párrafo con el espaciado por defecto, así que quedan muy separadas. Si docxgo lo permite (busca `SetSpacing` o similar en `domain.Paragraph`, en `~/go/pkg/mod/github.com/mmonterroca/docxgo/v2@v2.14.0`), pon el espaciado antes y después a 0 en esos párrafos.
Verifica también que la sangría inicial se conserve: el test `"```\n    x\n```"` tiene que dejar `    x` en `document.xml` con `xml:space="preserve"`. Si docxgo no lo hace, corrígelo.

## Criterio de terminado
- `go vet ./... && go test ./...` en verde y un commit por tarea en `fix/md-render`.
- `go run ./cmd/mkdocx build testdata/ejemplo.md -o /tmp/ej.docx` funciona.
- Responde con un resumen breve: tareas hechas, decisiones de diseño (en particular el modelo de la tarea 3) y cualquier cosa que no hayas podido hacer.
