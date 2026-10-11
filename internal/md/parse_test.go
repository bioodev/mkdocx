package md

import (
	"reflect"
	"strings"
	"testing"
)

func parse(t *testing.T, s string) Result {
	t.Helper()
	r, err := Parse([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestParseBlocks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []Block
	}{
		{"vacio", "", []Block{}},
		{"solo espacios", "  \n\n", []Block{}},
		{"heading", "# T", []Block{Heading{1, []Inline{{Text: "T"}}}}},
		{"formato", "a **b** *c* `d`", []Block{Paragraph{[]Inline{
			{Text: "a "}, {Text: "b", Bold: true}, {Text: " "}, {Text: "c", Italic: true}, {Text: " "}, {Text: "d", Code: true}}}}},
		{"enlace", "[x](http://u)", []Block{Paragraph{[]Inline{{Text: "x", URL: "http://u"}}}}},
		{"autoenlace", "<https://go.dev>", []Block{Paragraph{[]Inline{{Text: "https://go.dev", URL: "https://go.dev"}}}}},
		{"autoenlace correo", "<a@b.cl>", []Block{Paragraph{[]Inline{{Text: "a@b.cl", URL: "mailto:a@b.cl"}}}}},
		{"salto suave en enlace", "[foo\nbar](u)", []Block{Paragraph{[]Inline{{Text: "foo bar", URL: "u"}}}}},
		{"imagen", "![alt](p.png)", []Block{Image{Path: "p.png", Alt: "alt"}}},
		{"imagen remota", "![alt](https://x.com/i.png)", []Block{Paragraph{[]Inline{{Text: "alt", URL: "https://x.com/i.png"}}}}},
		{"pagebreak", "<!-- pagebreak -->", []Block{PageBreak{}}},
		{"codigo", "```go\nfmt.Println()\n```", []Block{CodeBlock{"fmt.Println()"}}},
		{"cita", "> hola", []Block{Quote{[]Block{Paragraph{[]Inline{{Text: "hola"}}}}}}},
		{"xml literal", "a < b & c é 😀", []Block{Paragraph{[]Inline{{Text: "a < b & c é 😀"}}}}},
		{"escapes y entidades", `\*no\* AT&amp;T &copy;`, []Block{Paragraph{[]Inline{{Text: "*no* AT&T ©"}}}}},
		{"codigo no desescapa", "`\\*x\\* &amp;`", []Block{Paragraph{[]Inline{{Text: `\*x\* &amp;`, Code: true}}}}},
		{"lista anidada", "- a\n  - b\n- c", []Block{List{Items: []ListItem{
			{Inlines: []Inline{{Text: "a"}}, Blocks: []Block{List{Items: []ListItem{{Inlines: []Inline{{Text: "b"}}}}}}},
			{Inlines: []Inline{{Text: "c"}}}}}}},
		{"lista ordenada", "1. a\n2. b", []Block{List{Ordered: true, Start: 1, Items: []ListItem{
			{Inlines: []Inline{{Text: "a"}}}, {Inlines: []Inline{{Text: "b"}}}}}}},
		{"codigo en item", "- a\n\n  ```\n  code\n  ```", []Block{List{Items: []ListItem{
			{Inlines: []Inline{{Text: "a"}}, Blocks: []Block{CodeBlock{"code"}}}}}}},
		{"lista dentro de cita", "> - x\n> - y", []Block{Quote{[]Block{List{Items: []ListItem{
			{Inlines: []Inline{{Text: "x"}}}, {Inlines: []Inline{{Text: "y"}}}}}}}}},
		{"items loose", "- uno\n\n  dos\n\n- tres", []Block{List{Items: []ListItem{
			{Inlines: []Inline{{Text: "uno"}}, Blocks: []Block{Paragraph{[]Inline{{Text: "dos"}}}}},
			{Inlines: []Inline{{Text: "tres"}}}}}}},
		{"tabla", "| A | B |\n|---|---|\n| 1 | 2 |", []Block{Table{
			Header: [][]Inline{{{Text: "A"}}, {{Text: "B"}}},
			Rows:   [][][]Inline{{{{Text: "1"}}, {{Text: "2"}}}}}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parse(t, c.in).Blocks
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %#v\nwant %#v", got, c.want)
			}
		})
	}
}

func TestFrontMatter(t *testing.T) {
	t.Run("linea en blanco no es front matter", func(t *testing.T) {
		r := parse(t, "---\n\nIntro\n\n---\n\nmore")
		if len(r.Blocks) == 0 || !reflect.DeepEqual(r.Blocks[0], Paragraph{[]Inline{{Text: "Intro"}}}) {
			t.Fatalf("se perdió Intro: %#v", r.Blocks)
		}
		if !strings.Contains(strings.Join(r.Warnings, "|"), "regla horizontal") {
			t.Errorf("faltan avisos: %v", r.Warnings)
		}
	})
	t.Run("front matter válido", func(t *testing.T) {
		r := parse(t, "---\ntitle: x\n---\n# T")
		if !strings.Contains(strings.Join(r.Warnings, "|"), "front matter") {
			t.Errorf("faltan avisos: %v", r.Warnings)
		}
		if len(r.Blocks) != 1 || !reflect.DeepEqual(r.Blocks[0], Heading{1, []Inline{{Text: "T"}}}) {
			t.Fatalf("bloques inesperados: %#v", r.Blocks)
		}
	})
	t.Run("front matter con CRLF", func(t *testing.T) {
		r := parse(t, "---\r\ntitle: x\r\n---\r\n# T")
		if !strings.Contains(strings.Join(r.Warnings, "|"), "front matter") {
			t.Errorf("faltan avisos: %v", r.Warnings)
		}
	})
	t.Run("cierre inválido no es front matter", func(t *testing.T) {
		r := parse(t, "---\na: b\n----x")
		if strings.Contains(strings.Join(r.Warnings, "|"), "front matter") {
			t.Errorf("no debería ser front matter: %v", r.Warnings)
		}
	})
}

func TestImagenRemotaAvisa(t *testing.T) {
	r := parse(t, "![a](https://x.com/i.png)")
	if !strings.Contains(strings.Join(r.Warnings, "|"), "remota") {
		t.Errorf("falta aviso de imagen remota: %v", r.Warnings)
	}
}

func TestParseWarnings(t *testing.T) {
	r := parse(t, "---\ntitle: x\n---\n\n# Hola\n\n<div>x</div>\n\n---\n\n~~t~~")
	joined := strings.Join(r.Warnings, "|")
	for _, want := range []string{"front matter", "HTML", "regla horizontal", "tachado"} {
		if !strings.Contains(joined, want) {
			t.Errorf("falta aviso %q en %q", want, joined)
		}
	}
	if len(r.Blocks) == 0 || !reflect.DeepEqual(r.Blocks[0], Heading{1, []Inline{{Text: "Hola"}}}) {
		t.Fatalf("primer bloque inesperado: %#v", r.Blocks)
	}
}
