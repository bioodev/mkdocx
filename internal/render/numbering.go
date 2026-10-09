package render

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// numIDBullet es la lista con viñetas; las listas ordenadas usan numId 2, 3, ...
// (una instancia por lista para que cada una reinicie en 1).
const numIDBullet = 1

func numberingXML(ordered int) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)
	for abs, spec := range []struct{ fmtName, text string }{{"bullet", "•"}, {"decimal", ""}} {
		fmt.Fprintf(&b, `<w:abstractNum w:abstractNumId="%d"><w:multiLevelType w:val="hybridMultilevel"/>`, abs)
		for lvl := 0; lvl < 9; lvl++ {
			text := spec.text
			if spec.fmtName == "decimal" {
				text = fmt.Sprintf("%%%d.", lvl+1)
			}
			fmt.Fprintf(&b, `<w:lvl w:ilvl="%d"><w:start w:val="1"/><w:numFmt w:val="%s"/><w:lvlText w:val="%s"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="%d" w:hanging="360"/></w:pPr></w:lvl>`,
				lvl, spec.fmtName, text, 720*(lvl+1))
		}
		b.WriteString(`</w:abstractNum>`)
	}
	b.WriteString(`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`)
	for i := 0; i < ordered; i++ {
		fmt.Fprintf(&b, `<w:num w:numId="%d"><w:abstractNumId w:val="1"/>`, i+2)
		for lvl := 0; lvl < 9; lvl++ {
			fmt.Fprintf(&b, `<w:lvlOverride w:ilvl="%d"><w:startOverride w:val="1"/></w:lvlOverride>`, lvl)
		}
		b.WriteString(`</w:num>`)
	}
	b.WriteString(`</w:numbering>`)
	return b.String()
}

// injectNumbering agrega word/numbering.xml (docxgo no lo genera en documentos nuevos)
// junto con su content-type y su relación.
func injectNumbering(src []byte, ordered int) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		switch f.Name {
		case "[Content_Types].xml":
			data = []byte(strings.Replace(string(data), "</Types>",
				`<Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/></Types>`, 1))
		case "word/_rels/document.xml.rels":
			data = []byte(strings.Replace(string(data), "</Relationships>",
				`<Relationship Id="rIdNum1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/></Relationships>`, 1))
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}
	w, err := zw.Create("word/numbering.xml")
	if err != nil {
		return nil, err
	}
	if _, err := w.Write([]byte(numberingXML(ordered))); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
