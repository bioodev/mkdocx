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

// ListItem es un ítem de lista. El primer párrafo del ítem va en Inlines y el
// resto de bloques (código, tablas, listas anidadas, párrafos extra...) en
// Blocks, en el mismo orden en que aparecen.
type ListItem struct {
	Inlines []Inline
	Blocks  []Block
}
type List struct {
	Ordered bool
	Start   int
	Items   []ListItem
}
type CodeBlock struct{ Text string }

// Quote es una cita; se guardan sus bloques para no perder código, tablas o
// listas anidadas.
type Quote struct{ Blocks []Block }
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
