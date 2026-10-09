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
		{"imagen", "![alt](p.png)", []Block{Image{Path: "p.png", Alt: "alt"}}},
		{"pagebreak", "<!-- pagebreak -->", []Block{PageBreak{}}},
		{"codigo", "```go\nfmt.Println()\n```", []Block{CodeBlock{"fmt.Println()"}}},
		{"cita", "> hola", []Block{Quote{[]Inline{{Text: "hola"}}}}},
		{"xml literal", "a < b & c é 😀", []Block{Paragraph{[]Inline{{Text: "a < b & c é 😀"}}}}},
		{"lista anidada", "- a\n  - b\n- c", []Block{List{Items: []ListItem{
			{Inlines: []Inline{{Text: "a"}}, Children: &List{Items: []ListItem{{Inlines: []Inline{{Text: "b"}}}}}},
			{Inlines: []Inline{{Text: "c"}}}}}}},
		{"lista ordenada", "1. a\n2. b", []Block{List{Ordered: true, Items: []ListItem{
			{Inlines: []Inline{{Text: "a"}}}, {Inlines: []Inline{{Text: "b"}}}}}}},
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
