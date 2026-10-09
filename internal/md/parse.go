package md

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

const pageBreakMarker = "<!-- pagebreak -->"

// Result agrupa los bloques y los avisos de elementos no soportados.
type Result struct {
	Blocks   []Block
	Warnings []string
}

// Parse convierte Markdown en bloques. Nunca devuelve nil en Blocks.
func Parse(src []byte) (Result, error) {
	res := Result{Blocks: []Block{}}
	src = stripFrontMatter(src, &res)
	md := goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough))
	root := md.Parser().Parse(text.NewReader(src))
	p := &parser{src: src, res: &res}
	res.Blocks = append(res.Blocks, p.blocks(root)...)
	return res, nil
}

func stripFrontMatter(src []byte, res *Result) []byte {
	if !bytes.HasPrefix(src, []byte("---\n")) && !bytes.HasPrefix(src, []byte("---\r\n")) {
		return src
	}
	rest := src[4:]
	if i := bytes.Index(rest, []byte("\n---")); i >= 0 {
		end := i + 4
		if j := bytes.IndexByte(rest[end:], '\n'); j >= 0 {
			end += j + 1
		} else {
			end = len(rest)
		}
		res.Warnings = append(res.Warnings, "front matter YAML ignorado")
		return rest[end:]
	}
	return src
}

type parser struct {
	src []byte
	res *Result
}

func (p *parser) warn(msg string) {
	for _, w := range p.res.Warnings {
		if w == msg {
			return
		}
	}
	p.res.Warnings = append(p.res.Warnings, msg)
}

func (p *parser) blocks(parent ast.Node) []Block {
	var out []Block
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		switch v := n.(type) {
		case *ast.Heading:
			out = append(out, Heading{Level: v.Level, Inlines: p.inlines(v, Inline{})})
		case *ast.Paragraph, *ast.TextBlock:
			if img, ok := p.soloImage(n); ok {
				out = append(out, img)
				continue
			}
			out = append(out, Paragraph{Inlines: p.inlines(n, Inline{})})
		case *ast.List:
			out = append(out, *p.list(v))
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			out = append(out, CodeBlock{Text: p.lines(n)})
		case *ast.Blockquote:
			var in []Inline
			for c := v.FirstChild(); c != nil; c = c.NextSibling() {
				if len(in) > 0 {
					in = append(in, Inline{Text: "\n"})
				}
				in = append(in, p.inlines(c, Inline{})...)
			}
			out = append(out, Quote{Inlines: in})
		case *extast.Table:
			out = append(out, p.table(v))
		case *ast.HTMLBlock:
			raw := strings.TrimSpace(p.lines(v))
			if raw == pageBreakMarker {
				out = append(out, PageBreak{})
			} else {
				p.warn("HTML embebido ignorado")
			}
		case *ast.ThematicBreak:
			p.warn("regla horizontal (---) ignorada; usa <!-- pagebreak --> para saltar de página")
		}
	}
	return out
}

func (p *parser) lines(n ast.Node) string {
	var b strings.Builder
	ls := n.Lines()
	for i := 0; i < ls.Len(); i++ {
		s := ls.At(i)
		b.Write(s.Value(p.src))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (p *parser) soloImage(n ast.Node) (Image, bool) {
	c := n.FirstChild()
	if c == nil || c.NextSibling() != nil {
		return Image{}, false
	}
	img, ok := c.(*ast.Image)
	if !ok {
		return Image{}, false
	}
	return Image{Path: string(img.Destination), Alt: p.plain(img)}, true
}

func (p *parser) plain(n ast.Node) string {
	var b strings.Builder
	for _, in := range p.inlines(n, Inline{}) {
		b.WriteString(in.Text)
	}
	return b.String()
}

func (p *parser) list(l *ast.List) *List {
	out := &List{Ordered: l.IsOrdered()}
	for it := l.FirstChild(); it != nil; it = it.NextSibling() {
		item := ListItem{}
		for c := it.FirstChild(); c != nil; c = c.NextSibling() {
			switch v := c.(type) {
			case *ast.List:
				item.Children = p.list(v)
			default:
				if len(item.Inlines) > 0 {
					item.Inlines = append(item.Inlines, Inline{Text: " "})
				}
				item.Inlines = append(item.Inlines, p.inlines(c, Inline{})...)
			}
		}
		out.Items = append(out.Items, item)
	}
	return out
}

func (p *parser) table(t *extast.Table) Table {
	out := Table{}
	for r := t.FirstChild(); r != nil; r = r.NextSibling() {
		var row [][]Inline
		for c := r.FirstChild(); c != nil; c = c.NextSibling() {
			row = append(row, p.inlines(c, Inline{}))
		}
		if _, ok := r.(*extast.TableHeader); ok {
			out.Header = row
		} else {
			out.Rows = append(out.Rows, row)
		}
	}
	return out
}

// inlines aplana los hijos de n y fusiona tramos contiguos con el mismo formato.
func (p *parser) inlines(n ast.Node, st Inline) []Inline {
	var out []Inline
	for _, in := range p.walk(n, st) {
		if k := len(out) - 1; k >= 0 && sameStyle(out[k], in) {
			out[k].Text += in.Text
			continue
		}
		out = append(out, in)
	}
	return out
}

func sameStyle(a, b Inline) bool {
	return a.Bold == b.Bold && a.Italic == b.Italic && a.Code == b.Code && a.URL == b.URL
}

// walk aplana los hijos de n acumulando el estilo heredado.
func (p *parser) walk(n ast.Node, st Inline) []Inline {
	var out []Inline
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *ast.Text:
			t := st
			t.Text = string(v.Segment.Value(p.src))
			out = append(out, t)
			if v.HardLineBreak() {
				out = append(out, Inline{Text: "\n"})
			} else if v.SoftLineBreak() {
				out = append(out, Inline{Text: " ", Bold: st.Bold, Italic: st.Italic})
			}
		case *ast.String:
			t := st
			t.Text = string(v.Value)
			out = append(out, t)
		case *ast.CodeSpan:
			t := st
			t.Code = true
			t.Text = p.plain(v)
			out = append(out, t)
		case *ast.Emphasis:
			s := st
			if v.Level >= 2 {
				s.Bold = true
			} else {
				s.Italic = true
			}
			out = append(out, p.walk(v, s)...)
		case *ast.Link:
			s := st
			s.URL = string(v.Destination)
			out = append(out, p.walk(v, s)...)
		case *ast.Image:
			t := st
			t.Text = p.plain(v)
			out = append(out, t)
			p.warn("imagen dentro de texto: se usa el texto alternativo")
		case *ast.RawHTML:
			p.warn("HTML embebido ignorado")
		case *extast.Strikethrough:
			p.warn("tachado no soportado: se muestra sin formato")
			out = append(out, p.walk(v, st)...)
		default:
			out = append(out, p.walk(c, st)...)
		}
	}
	return out
}
