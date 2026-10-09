// Package md convierte Markdown en un modelo de bloques independiente de docx.
package md

// Inline es un tramo de texto con formato.
type Inline struct {
	Text   string
	Bold   bool
	Italic bool
	Code   bool
	URL    string
}

// Block es un elemento de nivel de bloque.
type Block interface{ isBlock() }

type Heading struct {
	Level   int
	Inlines []Inline
}
type Paragraph struct{ Inlines []Inline }
type ListItem struct {
	Inlines  []Inline
	Children *List
}
type List struct {
	Ordered bool
	Items   []ListItem
}
type CodeBlock struct{ Text string }
type Quote struct{ Inlines []Inline }
type Table struct {
	Header [][]Inline
	Rows   [][][]Inline
}
type Image struct{ Path, Alt string }
type PageBreak struct{}

func (Heading) isBlock()   {}
func (Paragraph) isBlock() {}
func (List) isBlock()      {}
func (CodeBlock) isBlock() {}
func (Quote) isBlock()     {}
func (Table) isBlock()     {}
func (Image) isBlock()     {}
func (PageBreak) isBlock() {}
