package fill

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFlatten(t *testing.T) {
	data, err := ParseData(strings.NewReader(`{"a":{"b":1},"c":true,"d":"x","e":null,"n":12345678901234567890}`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Flatten(data)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"a.b": "1", "c": "true", "d": "x", "e": "", "n": "12345678901234567890"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestFlattenErrores(t *testing.T) {
	d, _ := ParseData(strings.NewReader(`{"a":[1,2]}`))
	if _, err := Flatten(d); err == nil {
		t.Error("arrays deberían rechazarse")
	}
	d, _ = ParseData(strings.NewReader(`{"a.b":"x","a":{"b":"y"}}`))
	if _, err := Flatten(d); err == nil {
		t.Error("colisión debería detectarse")
	}
	if _, err := ParseData(strings.NewReader(`[1]`)); err == nil {
		t.Error("JSON no objeto debería fallar")
	}
}

// plantilla construye un .docx mínimo cuyo placeholder está partido en tres runs,
// como hace Word al editar.
func plantilla(t *testing.T, body string) string {
	t.Helper()
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":         `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body + `</w:body></w:document>`,
	}
	path := filepath.Join(t.TempDir(), "t.docx")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, c := range files {
		w, _ := zw.Create(name)
		w.Write([]byte(c))
	}
	zw.Close()
	os.WriteFile(path, buf.Bytes(), 0o644)
	return path
}

func docXML(t *testing.T, b []byte) string {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			d, _ := io.ReadAll(rc)
			return string(d)
		}
	}
	t.Fatal("sin document.xml")
	return ""
}

func TestFillRunsPartidos(t *testing.T) {
	p := plantilla(t, `<w:p><w:r><w:t>Hola {{no</w:t></w:r><w:r><w:t>mbre</w:t></w:r><w:r><w:t>}} fin</w:t></w:r></w:p>`)
	var out bytes.Buffer
	if _, err := Fill(p, map[string]any{"nombre": "Ana"}, false, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(docXML(t, out.Bytes()), "Hola Ana fin") {
		t.Errorf("no se rellenó: %s", docXML(t, out.Bytes()))
	}
}

func TestFillEstricto(t *testing.T) {
	p := plantilla(t, `<w:p><w:r><w:t>{{falta}}</w:t></w:r></w:p>`)
	_, err := Fill(p, map[string]any{}, true, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "falta") {
		t.Fatalf("esperaba error nombrando la clave, got %v", err)
	}
}

func TestFillPlantillaInexistente(t *testing.T) {
	if _, err := Fill("/no/existe.docx", map[string]any{}, false, io.Discard); err == nil {
		t.Fatal("esperaba error")
	}
}

func TestFillAvisaMarcadorSinDato(t *testing.T) {
	p := plantilla(t, `<w:p><w:r><w:t>{{a}} {{falta}}</w:t></w:r></w:p>`)
	w, err := Fill(p, map[string]any{"a": "1"}, false, io.Discard)
	if err != nil || len(w) != 1 || !strings.Contains(w[0], "falta") {
		t.Fatalf("avisos=%v err=%v", w, err)
	}
}
