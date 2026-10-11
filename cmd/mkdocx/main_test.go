package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func exec(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func escribir(t *testing.T, name, content string) string {
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func esZipValido(t *testing.T, path string) {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("zip inválido: %v", err)
	}
	zr.Close()
}

func TestBuildArchivo(t *testing.T) {
	in := escribir(t, "in.md", "# Hola\n\ntexto")
	out := filepath.Join(t.TempDir(), "out.docx")
	code, so, se := exec(t, "", "build", in, "-o", out)
	if code != 0 || so != out+"\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, so, se)
	}
	esZipValido(t, out)
}

func TestBuildFlagsDespues(t *testing.T) {
	in := escribir(t, "in.md", "x")
	out := filepath.Join(t.TempDir(), "o.docx")
	if code, _, se := exec(t, "", "build", "-o", out, in, "--title", "T"); code != 0 {
		t.Fatalf("code=%d %s", code, se)
	}
}

func TestBuildStdinStdout(t *testing.T) {
	code, so, _ := exec(t, "# hola", "build", "-", "-o", "-")
	if code != 0 || !strings.HasPrefix(so, "PK") {
		t.Fatalf("code=%d, salida no es zip", code)
	}
}

// xmlBienFormado parsea todos los tokens para comprobar que el XML es válido.
func xmlBienFormado(t *testing.T, name, s string) {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(s))
	for {
		if _, err := d.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("XML inválido en %s: %v", name, err)
		}
	}
}

// TestBuildEjemploEndToEnd genera el documento de testdata/ejemplo.md y valida
// el .docx completo: zip correcto, partes XML/rels bien formadas, contenido,
// numbering y al menos una imagen.
func TestBuildEjemploEndToEnd(t *testing.T) {
	out := filepath.Join(t.TempDir(), "ejemplo.docx")
	code, _, se := exec(t, "", "build", "../../testdata/ejemplo.md", "-o", out)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, se)
	}
	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatalf("zip inválido: %v", err)
	}
	defer zr.Close()

	var doc string
	tieneNumbering := false
	var media []string
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(f.Name, ".xml") || strings.HasSuffix(f.Name, ".rels") {
			xmlBienFormado(t, f.Name, string(data))
		}
		switch {
		case f.Name == "word/document.xml":
			doc = string(data)
		case f.Name == "word/numbering.xml":
			tieneNumbering = true
		case strings.HasPrefix(f.Name, "word/media/"):
			media = append(media, f.Name)
		}
	}
	if !strings.Contains(doc, "Informe de ejemplo") || !strings.Contains(doc, "Segunda página") {
		t.Error("word/document.xml no contiene 'Informe de ejemplo' y 'Segunda página'")
	}
	if !tieneNumbering {
		t.Error("falta word/numbering.xml")
	}
	if len(media) == 0 {
		t.Error("no hay archivos en word/media/")
	}
}

func TestBuildErrores(t *testing.T) {
	tmp := t.TempDir()
	casos := []struct {
		name string
		args []string
		want string
	}{
		{"entrada inexistente", []string{"build", "/no/existe.md", "-o", filepath.Join(tmp, "o.docx")}, "/no/existe.md"},
		{"sin -o", []string{"build", "x.md"}, "uso"},
		{"salida en dir inexistente", []string{"build", escribir(t, "a.md", "x"), "-o", "/no/dir/o.docx"}, "/no/dir"},
		{"imagen faltante", []string{"build", escribir(t, "b.md", "![x](nada.png)"), "-o", filepath.Join(tmp, "o.docx")}, "nada.png"},
		{"subcomando", []string{"nada"}, "desconocido"},
		{"sin args", nil, "Uso"},
	}
	for _, c := range casos {
		t.Run(c.name, func(t *testing.T) {
			code, _, se := exec(t, "", c.args...)
			if code != 1 || !strings.Contains(se, c.want) {
				t.Fatalf("code=%d stderr=%q, esperaba exit 1 con %q", code, se, c.want)
			}
		})
	}
}

func TestBuildAvisos(t *testing.T) {
	in := escribir(t, "in.md", "---\na: b\n---\n# T")
	out := filepath.Join(t.TempDir(), "o.docx")
	code, _, se := exec(t, "", "build", in, "-o", out)
	if code != 0 || !strings.Contains(se, "aviso: front matter") {
		t.Fatalf("code=%d stderr=%q", code, se)
	}
}

func TestVersion(t *testing.T) {
	if code, so, _ := exec(t, "", "--version"); code != 0 || !strings.Contains(so, "mkdocx") {
		t.Fatalf("code=%d %q", code, so)
	}
}

// TestVersionEfectiva cubre la resolución de versión: con -ldflags manda la
// inyectada y, sin ella, debe caer en la del build info o en "dev", nunca en
// "(devel)" ni vacío.
func TestVersionEfectiva(t *testing.T) {
	orig := version
	defer func() { version = orig }()

	version = "v1.2.3"
	if got := versionEfectiva(); got != "v1.2.3" {
		t.Errorf("con -ldflags: got %q, esperaba v1.2.3", got)
	}

	version = "dev"
	if got := versionEfectiva(); got == "" || got == "(devel)" {
		t.Errorf("sin -ldflags: got %q, esperaba una versión válida", got)
	}
}

func TestFillEndToEnd(t *testing.T) {
	tmp := t.TempDir()
	md := escribir(t, "t.md", "Hola {{nombre}}")
	tpl := filepath.Join(tmp, "t.docx")
	if code, _, se := exec(t, "", "build", md, "-o", tpl); code != 0 {
		t.Fatal(se)
	}
	out := filepath.Join(tmp, "o.docx")
	code, _, se := exec(t, `{"nombre":"Ana"}`, "fill", tpl, "--data", "-", "-o", out)
	if code != 0 {
		t.Fatalf("code=%d %s", code, se)
	}
	esZipValido(t, out)
	// strict con clave faltante -> exit 1 nombrando la clave
	code, _, se = exec(t, `{}`, "fill", tpl, "--data", "-", "-o", out, "--strict")
	if code != 1 || !strings.Contains(se, "nombre") {
		t.Fatalf("code=%d stderr=%q", code, se)
	}
	// JSON inválido -> exit 1
	if code, _, _ := exec(t, `{`, "fill", tpl, "--data", "-", "-o", out); code != 1 {
		t.Fatalf("JSON inválido: code=%d", code)
	}
	// plantilla inexistente -> exit 1
	if code, _, _ := exec(t, `{}`, "fill", "/no/t.docx", "--data", "-", "-o", out); code != 1 {
		t.Fatalf("plantilla inexistente: code=%d", code)
	}
}
