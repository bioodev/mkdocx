// mkdocx genera documentos .docx desde Markdown o rellena plantillas con JSON.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/bioodev/mkdocx/internal/fill"
)

var version = "dev"

const usage = `mkdocx - genera documentos .docx

Uso:
  mkdocx build <entrada.md|-> -o <salida.docx|-> [--title T] [--author A]
  mkdocx fill  <plantilla.docx> --data <datos.json|-> -o <salida.docx|-> [--strict]
  mkdocx --version

"-" significa stdin/stdout. Con -o archivo, stdout imprime solo la ruta generada.
Códigos de salida: 0 ok, 1 uso o entrada inválida, 2 fallo al generar.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 1
	}
	var err error
	switch args[0] {
	case "--version", "-v", "version":
		fmt.Fprintln(stdout, "mkdocx", version)
		return 0
	case "--help", "-h", "help":
		fmt.Fprint(stdout, usage)
		return 0
	case "build":
		err = cmdBuild(args[1:], stdin, stdout, stderr)
	case "fill":
		err = cmdFill(args[1:], stdin, stdout, stderr)
	default:
		err = userErr("subcomando desconocido %q\n%s", args[0], usage)
	}
	return report(err, stderr)
}

// usageError es un error del usuario (exit 1); cualquier otro es fallo interno (exit 2).
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func userErr(format string, a ...any) error { return &usageError{fmt.Sprintf(format, a...)} }

func report(err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	fmt.Fprintln(stderr, "mkdocx:", err)
	var ue *usageError
	var ie *fill.InputError
	if errors.As(err, &ue) || errors.As(err, &ie) {
		return 1
	}
	return 2
}

// writeOutput escribe data en out ("-" = stdout) de forma atómica para archivos.
func writeOutput(out string, data []byte, stdout io.Writer) error {
	if out == "-" {
		_, err := stdout.Write(data)
		return err
	}
	dir := filepath.Dir(out)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return userErr("directorio de salida inexistente: %s", dir)
	}
	tmp, err := os.CreateTemp(dir, ".mkdocx-*.tmp")
	if err != nil {
		return userErr("no se puede escribir en %s: %v", dir, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), out); err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, out)
	return err
}

func readInput(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(stdin)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, userErr("no se puede leer %s: %v", path, err)
	}
	return b, nil
}
