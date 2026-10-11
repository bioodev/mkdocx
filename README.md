# mkdocx

CLI en Go, liviano y sin dependencias de runtime, que genera `.docx`:

```
mkdocx build entrada.md -o salida.docx [--title T] [--author A]
mkdocx fill  plantilla.docx --data datos.json -o salida.docx [--strict]
```

- Binario estático de ~5 MB (`make size` falla si supera 10 MB).
- Dependencias: [docxgo](https://github.com/mmonterroca/docxgo) v2.14.0 y goldmark.
- Skill para agentes: [`skills/mkdocx/SKILL.md`](skills/mkdocx/SKILL.md).

[![skills.sh](https://skills.sh/b/bioodev/mkdocx)](https://skills.sh/bioodev/mkdocx)

## Instalar
```bash
npx skills add bioodev/mkdocx                     # skill para tu agente (Claude Code, Cursor, Codex…)
sh skills/mkdocx/scripts/install.sh               # binario desde el último release (Linux/macOS)
go install github.com/bioodev/mkdocx/cmd/mkdocx@latest   # alternativa con Go
```

## Compilar
```bash
make test      # vet + tests
make build     # bin/mkdocx
make release   # dist/ para linux/mac/windows (amd64+arm64) + SHA256SUMS
```

## Publicar un release
Desde `main` limpio y al día, con un token de GitHub (fine-grained, permiso *Contents: write* solo en este repo):
```bash
DRY_RUN=1 make publish VERSION=v0.1.0             # ensayo: tests + binarios, sin push ni API
GITHUB_TOKEN=… make publish VERSION=v0.1.0        # tag + push + release con binarios y SHA256SUMS
```
Si falla a medias, vuelve a ejecutarlo: reutiliza el tag y el release y sube solo lo que falta.

## Limitaciones
- Plantillas: solo `{{clave}}` / `{{a.b}}` planas (sin bucles ni arrays).
- Markdown: sin notas al pie, HTML embebido, fórmulas ni tachado.
- Imágenes: solo PNG y JPEG locales; las remotas se convierten en enlace.

## Códigos de salida
`0` ok · `1` uso o entrada inválida · `2` fallo interno.

Spec y plan: `docs/superpowers/`. Errores conocidos y plan de mejoras: [`docs/pruebas-y-mejoras.md`](docs/pruebas-y-mejoras.md).
