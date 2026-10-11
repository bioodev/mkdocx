package render

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/bioodev/mkdocx/internal/md"
)

func build(t *testing.T, src, dir string) map[string]string {
	t.Helper()
	res, err := md.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Render(res.Blocks, dir, Meta{Title: "T", Author: "A"})
	if err != nil {
		t.Fatal(err)
	}
	return unzip(t, b)
}

func unzip(t *testing.T, b []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		parts[f.Name] = string(data)
		if strings.HasSuffix(f.Name, ".xml") || strings.HasSuffix(f.Name, ".rels") {
			d := xml.NewDecoder(strings.NewReader(string(data)))
			for {
				if _, err := d.Token(); err == io.EOF {
					break
				} else if err != nil {
					t.Fatalf("XML inválido en %s: %v", f.Name, err)
				}
			}
		}
	}
	return parts
}

func TestRenderCompleto(t *testing.T) {
	src := "# Hola\n\nTexto **negrita** y *cursiva* y [enlace](http://x.com).\n\n- a\n  - b\n    - c\n\n1. uno\n2. dos\n\n| A | B |\n|---|---|\n| 1 |\n\n```\ncodigo\n```\n\n> cita\n\n<!-- pagebreak -->\n\nfin"
	p := build(t, src, ".")
	doc := p["word/document.xml"]
	for _, want := range []string{"Heading1", "<w:b", "<w:i", "<w:tbl>", "Consolas", "negrita", "w:numPr", `w:br w:type="page"`, "hyperlink"} {
		if !strings.Contains(doc, want) {
			t.Errorf("document.xml no contiene %q", want)
		}
	}
	if _, ok := p["word/numbering.xml"]; !ok {
		t.Fatal("falta word/numbering.xml")
	}
	if !strings.Contains(p["[Content_Types].xml"], "numbering+xml") {
		t.Error("falta content-type de numbering")
	}
	if !strings.Contains(p["word/_rels/document.xml.rels"], "numbering.xml") {
		t.Error("falta relación de numbering")
	}
}

func TestRenderVacio(t *testing.T) {
	p := build(t, "  \n", ".")
	if !strings.Contains(p["word/document.xml"], "<w:p") {
		t.Error("esperaba al menos un párrafo")
	}
}

func TestRenderEscapaXML(t *testing.T) {
	doc := build(t, "a < b & c é 😀", ".")["word/document.xml"]
	if !strings.Contains(doc, "a &lt; b &amp; c é 😀") {
		t.Errorf("texto no preservado: %s", doc)
	}
}

func TestRenderListasOrdenadasReinician(t *testing.T) {
	p := build(t, "1. a\n2. b\n\ntexto\n\n1. c\n", ".")
	n := p["word/numbering.xml"]
	if !strings.Contains(n, `w:numId="2"`) || !strings.Contains(n, `w:numId="3"`) {
		t.Errorf("esperaba dos instancias de lista ordenada: %s", n)
	}
}

func TestRenderImagenFaltante(t *testing.T) {
	res, _ := md.Parse([]byte("![x](no-existe.png)"))
	_, err := Render(res.Blocks, "/tmp", Meta{})
	if err == nil || !strings.Contains(err.Error(), "no-existe.png") {
		t.Fatalf("error esperado con la ruta, got %v", err)
	}
}

func TestRenderImagenFormatoInvalido(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.png"), []byte("no soy png"), 0o644)
	res, _ := md.Parse([]byte("![x](a.png)"))
	if _, err := Render(res.Blocks, dir, Meta{}); err == nil || !strings.Contains(err.Error(), "PNG") {
		t.Fatalf("esperaba error de formato, got %v", err)
	}
}

// parrafos devuelve el texto de cada párrafo de document.xml, en orden.
func parrafos(t *testing.T, doc string) []string {
	t.Helper()
	reText := regexp.MustCompile(`<w:t[^>]*>(.*?)</w:t>`)
	var out []string
	for _, m := range regexp.MustCompile(`(?s)<w:p(?:>| )`).FindAllStringIndex(doc, -1) {
		end := strings.Index(doc[m[0]:], "</w:p>")
		if end < 0 {
			continue
		}
		var b strings.Builder
		for _, tm := range reText.FindAllStringSubmatch(doc[m[0]:m[0]+end], -1) {
			b.WriteString(tm[1])
		}
		out = append(out, b.String())
	}
	return out
}

func hayParrafo(ps []string, want string) bool {
	for _, p := range ps {
		if strings.Contains(p, want) {
			return true
		}
	}
	return false
}

func TestRenderCodigoDentroDeItem(t *testing.T) {
	doc := build(t, "- a\n\n  ```\n  code\n  ```\n", ".")["word/document.xml"]
	if !strings.Contains(doc, "code") {
		t.Fatalf("se perdió el bloque de código del ítem: %s", doc)
	}
}

func TestRenderListaDentroDeCita(t *testing.T) {
	doc := build(t, "> - x\n> - y\n", ".")["word/document.xml"]
	ps := parrafos(t, doc)
	var xs []string
	for _, p := range ps {
		if p == "x" || p == "y" {
			xs = append(xs, p)
		}
	}
	if len(xs) != 2 {
		t.Fatalf("esperaba x e y en párrafos distintos, ps=%q", ps)
	}
}

func TestRenderItemsLoose(t *testing.T) {
	doc := build(t, "- uno\n\n  dos\n\n- tres\n", ".")["word/document.xml"]
	ps := parrafos(t, doc)
	for _, want := range []string{"uno", "dos", "tres"} {
		if !hayParrafo(ps, want) {
			t.Fatalf("falta el párrafo %q en %q", want, ps)
		}
	}
}

func TestRenderCitaConBloques(t *testing.T) {
	doc := build(t, "> cita\n>\n> ```\n> z\n> ```\n", ".")["word/document.xml"]
	if !strings.Contains(doc, "cita") || !strings.Contains(doc, "z") {
		t.Fatalf("se perdió contenido de la cita: %s", doc)
	}
}
