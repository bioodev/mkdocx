# Plan B — Correcciones en el CLI, `fill` y el módulo

Rama: `fix/cli-fill` · Worktree: `../mkdocx-wt-cli`

## Reglas
- **Solo puedes tocar:** `cmd/mkdocx/main.go`, `cmd/mkdocx/fill.go`, `cmd/mkdocx/main_test.go`, `internal/fill/*`, `go.mod`, `go.sum` y archivos nuevos en `testdata/`.
  Otro agente trabaja en paralelo sobre `internal/md/*`, `internal/render/*`, `cmd/mkdocx/args.go` y `cmd/mkdocx/build.go`. No los toques. Tampoco `README.md` ni `skill/SKILL.md`.
- Go: `export PATH=$HOME/.local/go/bin:$PATH` (Go 1.27.2).
- TDD: para cada punto, primero escribe el test que falla, comprueba que falla y después corrige.
- Respeta el estilo del código: comentarios y mensajes en español, funciones cortas y sin dependencias nuevas.
- Al terminar cada punto, ejecuta `go vet ./... && go test ./...` y haz un commit en la rama con un mensaje convencional (`fix(cli): ...`).
- No hagas push.

## Tareas (en este orden)

### 1. `go mod tidy`
`go.mod` marca goldmark y docxgo como `// indirect` aunque se importan directamente. Ejecuta `go mod tidy` y haz el commit.

### 2. Permisos del archivo de salida (`writeOutput`, `main.go`)
`os.CreateTemp` crea el archivo como 0600 y `Rename` mantiene ese modo, así que el `.docx` acaba con permisos 0600.
Antes del `Rename`:
- si `out` ya existe, conserva su modo;
- si no, aplica `0o666` filtrado por la umask del proceso, igual que `os.Create`.

  Para obtener la umask de forma portable, la opción más sencilla es crear el archivo con `os.OpenFile(..., 0o666)` en lugar de `CreateTemp`, usando un nombre temporal único en el mismo directorio y `O_EXCL`. Elige la solución más simple que funcione también en Windows.

Test: con `syscall.Umask(0o022)` en el test (solo en Unix, con build tag o `runtime.GOOS`), el archivo generado tiene modo `0644`. Si existía con `0640`, lo conserva.

### 3. Versión con `go install`
Si `version == "dev"`, usa `debug.ReadBuildInfo()` y toma `Main.Version` cuando no sea `"(devel)"` ni esté vacío. Los releases siguen inyectando la versión por `-ldflags`.

### 4. Clasificación de errores en `fill.Fill`
Ahora cualquier error de `template.MergeTemplate` se trata como `InputError` (exit 1). Revisa en `~/go/pkg/mod/github.com/mmonterroca/docxgo/v2@v2.14.0/pkg/template` qué errores devuelve:
- las claves faltantes en modo estricto y los marcadores mal formados son error de entrada (exit 1);
- el resto es error interno (exit 2, `fmt.Errorf("combinar plantilla: %w", err)`).

Si docxgo no permite distinguirlos de forma fiable (por ejemplo, con tipos o `errors.Is`), deja constancia en un comentario y conserva el comportamiento actual.

### 5. Escapado de valores en `fill`
Test: plantilla con `{{v}}` y datos `{"v":"a<b & \"c\" 'd'"}`. El `document.xml` resultante tiene que ser XML válido (parsea todos los tokens, como `unzip` en `render_test.go`) y contener el texto correcto una vez decodificado.
Prueba también con un valor con salto de línea (`"l1\nl2"`): como mínimo, XML válido y sin pérdida de texto. Si docxgo no escapa, corrígelo escapando antes de llamar a `MergeTemplate`, siempre que eso no provoque un doble escape.

### 6. Test de extremo a extremo con `testdata/ejemplo.md`
En `main_test.go`, ejecuta `run([]string{"build", "../../testdata/ejemplo.md", "-o", tmp})` y comprueba:
- exit 0;
- que el zip es válido y todas las partes `.xml` y `.rels` parsean;
- que `word/document.xml` contiene `Informe de ejemplo` y `Segunda página`;
- que existe `word/numbering.xml`;
- que hay al menos un archivo en `word/media/`.

No compruebes los avisos ni detalles del parser: el otro agente los está cambiando.

## Criterio de terminado
- `go vet ./... && go test ./...` en verde y un commit por tarea en `fix/cli-fill`.
- Responde con un resumen breve: tareas hechas, qué hace docxgo con el escapado y con los errores, y cualquier cosa que no hayas podido hacer.
