# mkdocx

CLI en Go, liviano y sin dependencias de runtime, que genera `.docx`:

```
mkdocx build entrada.md -o salida.docx [--title T] [--author A]
mkdocx fill  plantilla.docx --data datos.json -o salida.docx [--strict]
```

- Binario estático de ~5 MB (`make size` falla si supera 10 MB).
- Dependencias: [docxgo](https://github.com/mmonterroca/docxgo) v2.14.0 y goldmark.
- Skill para agentes: [`skill/SKILL.md`](skill/SKILL.md).

## Compilar
```bash
make test      # vet + tests
make build     # bin/mkdocx
make release   # dist/ para linux/mac/windows (amd64+arm64) + SHA256SUMS
```

## Limitaciones
- Plantillas: solo `{{clave}}` / `{{a.b}}` planas (sin bucles ni arrays).
- Markdown: sin notas al pie, HTML embebido, fórmulas ni tachado.
- Imágenes: solo PNG y JPEG locales; las remotas se convierten en enlace.

## Códigos de salida
`0` ok · `1` uso o entrada inválida · `2` fallo interno.

Spec y plan: `docs/superpowers/`. Errores conocidos y plan de mejoras: [`docs/pruebas-y-mejoras.md`](docs/pruebas-y-mejoras.md).
