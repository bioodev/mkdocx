package md

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
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

// stripFrontMatter aplica la regla de pandoc: el bloque YAML solo cuenta si la
// línea siguiente al `---` inicial no está en blanco y el cierre es una línea
// con exactamente `---` o `...`.
func stripFrontMatter(src []byte, res *Result) []byte {
	rest, ok := bytes.CutPrefix(src, []byte("---\r\n"))
	if !ok {
		rest, ok = bytes.CutPrefix(src, []byte("---\n"))
	}
	if !ok || len(rest) == 0 || rest[0] == '\n' || rest[0] == '\r' {
		return src
	}
	for len(rest) > 0 {
		line := rest
		if i := bytes.IndexByte(line, '\n'); i >= 0 {
			line = line[:i+1]
		}
		if isFrontMatterEnd(line) {
			res.Warnings = append(res.Warnings, "front matter YAML ignorado")
			return rest[len(line):]
		}
		rest = rest[len(line):]
	}
	return src
}

func isFrontMatterEnd(line []byte) bool {
	line = bytes.TrimSuffix(line, []byte("\n"))
	line = bytes.TrimSuffix(line, []byte("\r"))
	return bytes.Equal(line, []byte("---")) || bytes.Equal(line, []byte("..."))
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
			out = append(out, Quote{Blocks: p.blocks(v)})
		case *extast.Table:
			out = append(out, p.table(v))
		default:
			if blk, ok := p.misc(n); ok {
				out = append(out, blk)
			}
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
	for _, in := range p.rawInlines(n, Inline{}) {
		b.WriteString(in.Text)
	}
	return b.String()
}

func (p *parser) list(l *ast.List) *List {
	out := &List{Ordered: l.IsOrdered(), Start: l.Start}
	for it := l.FirstChild(); it != nil; it = it.NextSibling() {
		out.Items = append(out.Items, p.listItem(it))
	}
	return out
}

// listItem extrae el primer párrafo del ítem en Inlines y conserva el resto de
// bloques (incluidas las listas anidadas) en Blocks.
func (p *parser) listItem(it ast.Node) ListItem {
	item := ListItem{}
	first := true
	for c := it.FirstChild(); c != nil; c = c.NextSibling() {
		// El primer párrafo se queda como texto del ítem; los siguientes,
		// separados, evitan la pérdida de párrafos en listas "loose".
		if _, ok := c.(*ast.Paragraph); ok && first {
			item.Inlines = p.inlines(c, Inline{})
			first = false
			continue
		}
		if _, ok := c.(*ast.TextBlock); ok && first {
			item.Inlines = p.inlines(c, Inline{})
			first = false
			continue
		}
		first = false
		if blk, ok := p.listItemBlock(c); ok {
			item.Blocks = append(item.Blocks, blk)
		}
	}
	return item
}

// listItemBlock convierte un hijo de ítem (que no es el primer párrafo) en bloque.
func (p *parser) listItemBlock(c ast.Node) (Block, bool) {
	switch v := c.(type) {
	case *ast.List:
		return *p.list(v), true
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		return CodeBlock{Text: p.lines(c)}, true
	case *ast.Blockquote:
		return Quote{Blocks: p.blocks(c)}, true
	case *extast.Table:
		return p.table(v), true
	case *ast.Heading:
		return Heading{Level: v.Level, Inlines: p.inlines(v, Inline{})}, true
	case *ast.Paragraph, *ast.TextBlock:
		if img, ok := p.soloImage(c); ok {
			return img, true
		}
		return Paragraph{Inlines: p.inlines(c, Inline{})}, true
	}
	return nil, false
}

// misc traduce bloques sueltos que no tienen contenido propio (HTML, reglas).
func (p *parser) misc(c ast.Node) (Block, bool) {
	switch v := c.(type) {
	case *ast.HTMLBlock:
		if strings.TrimSpace(p.lines(v)) == pageBreakMarker {
			return PageBreak{}, true
		}
		p.warn("HTML embebido ignorado")
	case *ast.ThematicBreak:
		p.warn("regla horizontal (---) ignorada; usa <!-- pagebreak --> para saltar de página")
	}
	return nil, false
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
	return mergeInlines(p.walk(n, st))
}

// rawInlines es como inlines pero sin resolver escapes ni entidades (código).
func (p *parser) rawInlines(n ast.Node, st Inline) []Inline {
	return mergeInlines(p.walkRaw(n, st))
}

func mergeInlines(list []Inline) []Inline {
	var out []Inline
	for _, in := range list {
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

// unescapeText resuelve escapes (`\*`) y entidades (`&amp;`, `&#169;`) igual que
// el renderer HTML de goldmark. No se aplica dentro de código.
func unescapeText(src []byte) string {
	return string(util.ResolveEntityNames(util.ResolveNumericReferences(util.UnescapePunctuations(src))))
}

// autoLink convierte un `<https://x>` o `<a@b>` en un inline enlazado.
func (p *parser) autoLink(v *ast.AutoLink, st Inline) Inline {
	t := st
	t.Text = string(v.Label(p.src))
	t.URL = string(v.URL(p.src))
	if v.AutoLinkType == ast.AutoLinkEmail && !strings.HasPrefix(strings.ToLower(t.URL), "mailto:") {
		t.URL = "mailto:" + t.URL
	}
	return t
}

// walkRaw recorre n sin resolver escapes ni entidades; se usa dentro de `código`.
func (p *parser) walkRaw(n ast.Node, st Inline) []Inline {
	var out []Inline
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *ast.Text:
			t := st
			t.Text = string(v.Segment.Value(p.src))
			if v.HardLineBreak() {
				t.Text += "\n"
			} else if v.SoftLineBreak() {
				t.Text += " "
			}
			out = append(out, t)
		case *ast.String:
			t := st
			t.Text = string(v.Value)
			out = append(out, t)
		case *ast.CodeSpan:
			s := st
			s.Code = true
			out = append(out, p.rawInlines(v, s)...)
		case *ast.Emphasis:
			s := st
			if v.Level >= 2 {
				s.Bold = true
			} else {
				s.Italic = true
			}
			out = append(out, p.walkRaw(v, s)...)
		case *ast.Link:
			s := st
			s.URL = string(v.Destination)
			out = append(out, p.walkRaw(v, s)...)
		case *ast.AutoLink:
			out = append(out, p.autoLink(v, st))
		default:
			out = append(out, p.walkRaw(c, st)...)
		}
	}
	return out
}

// walk aplana los hijos de n acumulando el estilo heredado.
func (p *parser) walk(n ast.Node, st Inline) []Inline {
	var out []Inline
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *ast.Text:
			t := st
			t.Text = unescapeText(v.Segment.Value(p.src))
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
		case *ast.AutoLink:
			out = append(out, p.autoLink(v, st))
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
