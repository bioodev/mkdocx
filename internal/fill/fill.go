// Package fill rellena plantillas .docx con datos JSON usando {{clave}} / {{a.b}}.
package fill

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	docx "github.com/mmonterroca/docxgo/v2"
	"github.com/mmonterroca/docxgo/v2/pkg/template"
)

// InputError marca errores causados por la entrada del usuario (exit 1).
type InputError struct{ Err error }

func (e *InputError) Error() string { return e.Err.Error() }
func (e *InputError) Unwrap() error { return e.Err }

func inputErr(format string, a ...any) error { return &InputError{fmt.Errorf(format, a...)} }

// ParseData decodifica JSON conservando la precisión de los números.
func ParseData(r io.Reader) (map[string]any, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	var data map[string]any
	if err := dec.Decode(&data); err != nil {
		return nil, inputErr("JSON inválido: %v", err)
	}
	if data == nil {
		return nil, inputErr("JSON inválido: se esperaba un objeto")
	}
	return data, nil
}

// Flatten aplana objetos anidados a claves "a.b". null pasa a cadena vacía;
// los arrays se rechazan y las colisiones de clave son error.
func Flatten(data map[string]any) (map[string]string, error) {
	out := map[string]string{}
	var walk func(prefix string, v any) error
	walk = func(prefix string, v any) error {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				key := k
				if prefix != "" {
					key = prefix + "." + k
				}
				if err := walk(key, child); err != nil {
					return err
				}
			}
			return nil
		case []any:
			return inputErr("clave %q: los arrays no están soportados (usa claves planas)", prefix)
		}
		var s string
		switch x := v.(type) {
		case nil:
		case string:
			s = x
		case json.Number:
			s = x.String()
		default:
			s = fmt.Sprint(x)
		}
		if _, dup := out[prefix]; dup {
			return inputErr("clave duplicada tras aplanar: %q", prefix)
		}
		out[prefix] = s
		return nil
	}
	if err := walk("", data); err != nil {
		return nil, err
	}
	return out, nil
}

// claveErrorFaltantes es el prefijo con que docxgo v2.14.0 señala las claves
// faltantes en modo estricto. No exporta un tipo de error ni envuelve el suyo
// con errors.Is, así que esta es la única vía fiable de distinguirlo.
const claveErrorFaltantes = "template: missing keys:"

// clasificarErrorMerge decide el código de salida de un fallo de MergeTemplate:
// las claves faltantes en modo estricto son error de entrada (exit 1); el resto
// (consolidar runs, fijar texto) son fallos internos (exit 2). Los marcadores
// mal formados no llegan aquí: el patrón simplemente no casa y no dan error.
func clasificarErrorMerge(err error) error {
	if strings.HasPrefix(err.Error(), claveErrorFaltantes) {
		return inputErr("combinar plantilla: %v", err)
	}
	return fmt.Errorf("combinar plantilla: %w", err)
}

// Fill abre la plantilla, aplica los datos y escribe el docx en w.
// Con strict, un placeholder sin dato es error.
// Devuelve avisos por marcadores sin dato (solo en modo no estricto).
//
// Los valores se insertan sin escapar en el modelo: docxgo serializa el texto
// con encoding/xml (escapa <, &, comillas y convierte \n en <w:br/>), así que
// pre-escapar aquí provocaría doble escape.
func Fill(templatePath string, data map[string]any, strict bool, w io.Writer) ([]string, error) {
	flat, err := Flatten(data)
	if err != nil {
		return nil, err
	}
	doc, err := docx.OpenDocument(templatePath)
	if err != nil {
		return nil, inputErr("abrir plantilla %s: %v", templatePath, err)
	}
	var warnings []string
	if !strict {
		for _, ve := range template.ValidateTemplate(doc, template.MergeData(flat)) {
			if ve.Severity == template.SeverityError {
				warnings = append(warnings, "marcador sin dato, se deja sin cambios: {{"+ve.Key+"}}")
			}
		}
	}
	opts := template.DefaultMergeOptions()
	opts.StrictMode = strict
	if err := template.MergeTemplate(doc, template.MergeData(flat), opts); err != nil {
		return nil, clasificarErrorMerge(err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("escribir docx: %w", err)
	}
	_, err = w.Write(buf.Bytes())
	return warnings, err
}
