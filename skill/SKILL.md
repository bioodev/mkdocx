---
name: mkdocx
description: Crea documentos Word (.docx) desde Markdown o rellena plantillas .docx con datos JSON usando el CLI `mkdocx` (binario Go único, sin Word ni LibreOffice). Úsalo cuando el usuario pida generar, exportar o convertir contenido a .docx, un informe o carta en Word, o completar una plantilla con {{campos}}. No sirve para leer o editar .docx arbitrarios, ni para PDF.
---

# mkdocx

CLI Go liviano: Markdown → `.docx` y plantilla `.docx` + JSON → `.docx`.

## Instalación (si `mkdocx` no está en PATH)
```bash
go install github.com/bioodev/mkdocx/cmd/mkdocx@latest   # o descarga el binario del release
mkdocx --version
```

## Markdown → docx
```bash
mkdocx build informe.md -o informe.docx --title "Informe" --author "Ana"
cat informe.md | mkdocx build - -o -   > informe.docx     # stdin/stdout
```
Con `-o archivo`, stdout imprime solo la ruta generada. Los avisos (`aviso: ...`) van a stderr y no son errores.
Las imágenes relativas se resuelven respecto al `.md` (o al cwd si la entrada es `-`).

### Markdown soportado
Títulos `#`–`######`, párrafos, `**negrita**`, `*cursiva*`, `` `código` ``, enlaces, listas con viñetas y numeradas (anidadas, numeración real de Word), bloques de código, citas `>`, tablas GFM (la primera fila va en negrita), imágenes PNG/JPEG locales (se escalan al ancho de página) y saltos de página con `<!-- pagebreak -->`.

### No soportado (se avisa o se ignora)
Front matter YAML, HTML embebido, tachado, `---` (regla horizontal) e imágenes dentro de texto en línea se avisan en stderr y se ignoran. Notas al pie (`[^1]`) y fórmulas **no** se interpretan: salen como texto literal, sin aviso.

## Plantilla + datos
La plantilla es un `.docx` con marcadores `{{clave}}` o `{{cliente.nombre}}` (también en tablas, encabezados y pies).
```bash
echo '{"nombre":"Ana","cliente":{"empresa":"ACME"},"total":1250.5}' > datos.json
mkdocx fill plantilla.docx --data datos.json -o salida.docx [--strict]
```
- JSON anidado se aplana a `a.b`; números y booleanos se convierten a texto; `null` = vacío.
- Los arrays no se soportan (usa claves planas: `item1`, `item2`).
- Los marcadores van **sin espacios**: `{{total}}` sí, `{{ total }}` no se reemplaza (queda literal, sin aviso).
- Sin `--strict`, un marcador sin dato queda intacto y se avisa en stderr (`aviso: marcador sin dato ...`).
- `--strict` falla con exit 1 si falta un dato; el mensaje nombra la clave (p. ej. `template: missing keys: total`) y no se crea el archivo.
- Tip: para crear una plantilla rápida, escribe un `.md` con `{{campos}}` y conviértelo con `build`.

## Códigos de salida
`0` ok · `1` uso o entrada inválida (archivo inexistente, JSON inválido, imagen faltante, `--strict`) · `2` fallo interno al generar.

## Verificar el resultado
```bash
unzip -tq salida.docx          # el zip es válido
unzip -p salida.docx word/document.xml | grep -o '<w:t[^>]*>[^<]*'   # texto final, sin herramientas extra
soffice --headless --convert-to txt:Text salida.docx                 # opcional, si hay LibreOffice (el texto lleva BOM)
```
`mkdocx --version` imprime `mkdocx dev` en compilaciones locales y la etiqueta en los releases.
