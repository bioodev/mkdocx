package main

import (
	"flag"
	"io"
	"path/filepath"

	"github.com/bioodev/mkdocx/internal/md"
	"github.com/bioodev/mkdocx/internal/render"
)

func cmdBuild(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	out := fs.String("o", "", "archivo de salida (- = stdout)")
	title := fs.String("title", "", "título del documento")
	author := fs.String("author", "", "autor del documento")
	in, err := parse(fs, args)
	if err != nil {
		return err
	}
	if in == "" || *out == "" {
		return userErr("uso: mkdocx build <entrada.md|-> -o <salida.docx|->")
	}
	src, err := readInput(in, stdin)
	if err != nil {
		return err
	}
	res, err := md.Parse(src)
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		fprintln(stderr, "aviso:", w)
	}
	baseDir := "."
	if in != "-" {
		baseDir = filepath.Dir(in)
	}
	data, err := render.Render(res.Blocks, baseDir, render.Meta{Title: *title, Author: *author})
	if err != nil {
		return asUserIfInput(err)
	}
	return writeOutput(*out, data, stdout)
}
