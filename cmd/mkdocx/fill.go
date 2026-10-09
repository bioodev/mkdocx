package main

import (
	"bytes"
	"flag"
	"io"
	"os"

	"github.com/bioodev/mkdocx/internal/fill"
)

func cmdFill(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("fill", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	out := fs.String("o", "", "archivo de salida (- = stdout)")
	dataPath := fs.String("data", "", "JSON con los datos (- = stdin)")
	strict := fs.Bool("strict", false, "error si falta algún dato")
	tpl, err := parse(fs, args)
	if err != nil {
		return err
	}
	if tpl == "" || *out == "" || *dataPath == "" {
		return userErr("uso: mkdocx fill <plantilla.docx> --data <datos.json|-> -o <salida.docx|->")
	}
	if tpl == "-" {
		return userErr("la plantilla debe ser un archivo, no stdin")
	}
	if _, err := os.Stat(tpl); err != nil {
		return userErr("no se puede leer %s: %v", tpl, err)
	}
	raw, err := readInput(*dataPath, stdin)
	if err != nil {
		return err
	}
	data, err := fill.ParseData(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	warnings, err := fill.Fill(tpl, data, *strict, &buf)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fprintln(stderr, "aviso:", w)
	}
	return writeOutput(*out, buf.Bytes(), stdout)
}
