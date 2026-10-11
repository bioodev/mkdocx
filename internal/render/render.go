// Package render traduce el modelo de bloques de md a un documento docx.
// Es, junto con fill, el único paquete que importa docxgo.
package render

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg" // registra decodificador para image.DecodeConfig
	_ "image/png"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	docx "github.com/mmonterroca/docxgo/v2"
	"github.com/mmonterroca/docxgo/v2/domain"

	"github.com/bioodev/mkdocx/internal/md"
)

// Meta son los metadatos opcionales del documento.
type Meta struct{ Title, Author string }

// maxImageWidthPx y maxImageHeightPx son el área útil aproximada (6,5" x 9"
// a 96 dpi); las imágenes se escalan manteniendo la proporción.
const (
	maxImageWidthPx  = 624
	maxImageHeightPx = 864
)

// InputError marca los errores que dependen de la entrada del usuario (por
// ejemplo, imágenes que no existen o con formato no soportado).
type InputError struct{ Err error }

func (e *InputError) Error() string { return e.Err.Error() }
func (e *InputError) Unwrap() error { return e.Err }

type renderer struct {
	doc     domain.Document
	baseDir string
	ordered []int // inicio de cada lista ordenada emitida (una instancia de numeración cada una)
}

// Render devuelve el contenido de un .docx. Las rutas de imagen relativas se
// resuelven contra baseDir.
func Render(blocks []md.Block, baseDir string, meta Meta) ([]byte, error) {
	r := &renderer{doc: docx.NewDocument(), baseDir: baseDir}
	if meta.Title != "" || meta.Author != "" {
		if err := r.doc.SetMetadata(&domain.Metadata{Title: meta.Title, Creator: meta.Author}); err != nil {
			return nil, err
		}
	}
	if len(blocks) == 0 {
		if _, err := r.doc.AddParagraph(); err != nil {
			return nil, err
		}
	}
	for _, b := range blocks {
		if err := r.block(b); err != nil {
			return nil, err
		}
	}
	var buf bytes.Buffer
	if _, err := r.doc.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("escribir docx: %w", err)
	}
	return injectNumbering(buf.Bytes(), r.ordered)
}

func (r *renderer) block(b md.Block) error {
	switch v := b.(type) {
	case md.Heading:
		level := min(max(v.Level, 1), 6)
		p, err := r.doc.AddParagraph()
		if err != nil {
			return err
		}
		if err := p.SetStyle(fmt.Sprintf("Heading%d", level)); err != nil {
			return err
		}
		return r.inlines(p, v.Inlines)
	case md.Paragraph:
		p, err := r.doc.AddParagraph()
		if err != nil {
			return err
		}
		return r.inlines(p, v.Inlines)
	case md.Quote:
		return r.container(v.Blocks, domain.StyleIDQuote)
	case md.CodeBlock:
		return r.codeBlock(v)
	case md.List:
		numID := numIDBullet
		if v.Ordered {
			numID = r.newNumbering(v.Start)
		}
		return r.list(&v, numID, 0)
	case md.Table:
		return r.table(v)
	case md.Image:
		return r.image(v)
	case md.PageBreak:
		return r.doc.AddPageBreak()
	}
	return nil
}

// container renderiza bloques de un contenedor (cita o ítem de lista). Los
// párrafos reciben el estilo indicado; el resto de bloques se emiten en orden.
func (r *renderer) container(blocks []md.Block, style string) error {
	for _, b := range blocks {
		switch v := b.(type) {
		case md.Paragraph:
			p, err := r.doc.AddParagraph()
			if err != nil {
				return err
			}
			if style != "" {
				if err := p.SetStyle(style); err != nil {
					return err
				}
			}
			if err := r.inlines(p, v.Inlines); err != nil {
				return err
			}
		default:
			if err := r.block(b); err != nil {
				return err
			}
		}
	}
	return nil
}

// containerAt es como container pero anida las listas en un nivel de sangría.
func (r *renderer) containerAt(blocks []md.Block, level int) error {
	for _, b := range blocks {
		switch v := b.(type) {
		case md.Paragraph:
			if err := r.container([]md.Block{v}, ""); err != nil {
				return err
			}
		case md.List:
			numID := numIDBullet
			if v.Ordered {
				numID = r.newNumbering(v.Start)
			}
			if err := r.list(&v, numID, level+1); err != nil {
				return err
			}
		default:
			if err := r.block(b); err != nil {
				return err
			}
		}
	}
	return nil
}

// codeBlock emite cada línea del bloque como un párrafo con espaciado 0 (las
// líneas de código no deben quedar separadas).
func (r *renderer) codeBlock(b md.CodeBlock) error {
	for _, line := range strings.Split(b.Text, "\n") {
		p, err := r.doc.AddParagraph()
		if err != nil {
			return err
		}
		if err := p.SetSpacingBefore(0); err != nil {
			return err
		}
		if err := p.SetSpacingAfter(0); err != nil {
			return err
		}
		if err := r.inlines(p, []md.Inline{{Text: line, Code: true}}); err != nil {
			return err
		}
	}
	return nil
}

// newNumbering reserva una instancia de numeración nueva para una lista
// ordenada que empieza en start (1 por defecto).
func (r *renderer) newNumbering(start int) int {
	if start < 1 {
		start = 1
	}
	r.ordered = append(r.ordered, start)
	return len(r.ordered) + 1
}

func (r *renderer) list(l *md.List, numID, level int) error {
	lvl := min(level, 8)
	for _, it := range l.Items {
		p, err := r.doc.AddParagraph()
		if err != nil {
			return err
		}
		if err := p.SetStyle(domain.StyleIDListParagraph); err != nil {
			return err
		}
		if err := p.SetNumbering(domain.NumberingReference{ID: numID, Level: lvl}); err != nil {
			return err
		}
		if err := r.inlines(p, it.Inlines); err != nil {
			return err
		}
		if err := r.containerAt(it.Blocks, lvl); err != nil {
			return err
		}
	}
	return nil
}

func (r *renderer) inlines(p domain.Paragraph, ins []md.Inline) error {
	for _, in := range ins {
		var run domain.Run
		var err error
		if in.URL != "" {
			run, err = p.AddHyperlink(in.URL, in.Text)
		} else {
			run, err = p.AddRun()
			if err == nil {
				err = r.setText(run, in.Text)
			}
		}
		if err != nil {
			return err
		}
		if in.Bold {
			if err := run.SetBold(true); err != nil {
				return err
			}
		}
		if in.Italic {
			if err := run.SetItalic(true); err != nil {
				return err
			}
		}
		if in.Code {
			if err := run.SetFont(domain.Font{Name: "Consolas"}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *renderer) setText(run domain.Run, text string) error {
	for i, part := range strings.Split(text, "\n") {
		if i > 0 {
			if err := run.AddBreak(domain.BreakTypeLine); err != nil {
				return err
			}
		}
		if part == "" {
			continue
		}
		if i == 0 {
			if err := run.SetText(part); err != nil {
				return err
			}
		} else if err := run.AddText(part); err != nil {
			return err
		}
	}
	return nil
}

func (r *renderer) table(t md.Table) error {
	cols := len(t.Header)
	for _, row := range t.Rows {
		cols = max(cols, len(row))
	}
	rows := len(t.Rows)
	if t.Header != nil {
		rows++
	}
	if rows == 0 || cols == 0 {
		return nil
	}
	tbl, err := r.doc.AddTable(rows, cols)
	if err != nil {
		return err
	}
	if err := tbl.SetStyle(domain.TableStyleGrid); err != nil {
		return err
	}
	fill := func(i int, cells [][]md.Inline, bold bool) error {
		row, err := tbl.Row(i)
		if err != nil {
			return err
		}
		for j := 0; j < cols; j++ {
			cell, err := row.Cell(j)
			if err != nil {
				return err
			}
			p, err := cell.AddParagraph()
			if err != nil {
				return err
			}
			if j >= len(cells) {
				continue
			}
			ins := cells[j]
			if bold {
				ins = append([]md.Inline(nil), ins...)
				for k := range ins {
					ins[k].Bold = true
				}
			}
			if err := r.inlines(p, ins); err != nil {
				return err
			}
		}
		return nil
	}
	i := 0
	if t.Header != nil {
		if err := fill(0, t.Header, true); err != nil {
			return err
		}
		i = 1
	}
	for _, row := range t.Rows {
		if err := fill(i, row, false); err != nil {
			return err
		}
		i++
	}
	return nil
}

func (r *renderer) image(im md.Image) error {
	path := im.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.baseDir, path)
	}
	f, err := os.Open(path)
	if err != nil {
		// La ruta puede venir codificada (por ejemplo, espacios como %20).
		if dec, derr := url.PathUnescape(path); derr == nil && dec != path {
			if f2, err2 := os.Open(dec); err2 == nil {
				f, path, err = f2, dec, nil
			}
		}
	}
	if err != nil {
		return &InputError{fmt.Errorf("imagen no encontrada: %s", path)}
	}
	cfg, format, err := image.DecodeConfig(f)
	f.Close()
	if err != nil || (format != "png" && format != "jpeg") {
		return &InputError{fmt.Errorf("imagen no soportada (solo PNG y JPEG): %s", path)}
	}
	w, h := fitImage(cfg.Width, cfg.Height)
	p, err := r.doc.AddParagraph()
	if err != nil {
		return err
	}
	_, err = p.AddImageWithSize(path, domain.NewImageSize(w, h))
	return err
}

// fitImage reduce w x h para que quepa en el área útil manteniendo la proporción.
func fitImage(w, h int) (int, int) {
	if w <= maxImageWidthPx && h <= maxImageHeightPx {
		return w, h
	}
	nw, nh := w, h
	if w > maxImageWidthPx {
		nh = h * maxImageWidthPx / w
		nw = maxImageWidthPx
	}
	if nh > maxImageHeightPx {
		nw = nw * maxImageHeightPx / nh
		nh = maxImageHeightPx
	}
	return max(nw, 1), max(nh, 1)
}
