#!/bin/bash
# Publica un release en GitHub sin gh: test, binarios, tag y subida por la API.
#
# Uso:   GITHUB_TOKEN=... scripts/release.sh v0.1.0
#        DRY_RUN=1 scripts/release.sh v0.1.0   # todo menos push y API
#
# El token necesita permiso "Contents: write" sobre el repo. Si algo falla a
# medias, se puede volver a ejecutar: reutiliza el tag (si apunta a HEAD), el
# release (si existe) y sube solo los archivos que falten.
set -euo pipefail

VERSION=${1:-}
DRY_RUN=${DRY_RUN:-}
API=${GITHUB_API_URL:-https://api.github.com}
UPLOADS=${GITHUB_UPLOADS_URL:-https://uploads.github.com}

err() { echo "release: $*" >&2; exit 1; }
paso() { echo "==> $*"; }

[[ $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || err "uso: scripts/release.sh vX.Y.Z"
command -v jq >/dev/null || err "se necesita jq"
[[ -n $DRY_RUN || -n ${GITHUB_TOKEN:-} ]] || err "falta GITHUB_TOKEN (o usa DRY_RUN=1)"

cd "$(git rev-parse --show-toplevel)"
REPO=$(git remote get-url origin | sed -E 's#.*[:/]([^/]+/[^/]+)$#\1#; s#\.git$##')
[[ $REPO == */* ]] || err "no se pudo deducir owner/repo del remoto origin"

paso "comprobando el repositorio ($REPO)"
[[ $(git branch --show-current) == main ]] || err "hay que publicar desde main"
[[ -z $(git status --porcelain) ]] || err "hay cambios sin commitear"
git fetch -q origin main --tags
[[ $(git rev-parse HEAD) == $(git rev-parse origin/main) ]] || err "main no está al día con origin/main"
if git rev-parse -q --verify "refs/tags/$VERSION" >/dev/null; then
	[[ $(git rev-list -n1 "$VERSION") == $(git rev-parse HEAD) ]] || err "el tag $VERSION ya existe y no apunta a HEAD"
	paso "el tag $VERSION ya existe en HEAD: se reutiliza"
	nuevo_tag=
else
	nuevo_tag=1
fi

paso "tests"
make test
paso "binarios"
make release VERSION="$VERSION"
host=dist/mkdocx-$(go env GOOS)-$(go env GOARCH)
if [[ -x $host ]]; then
	[[ $("$host" --version) == "mkdocx $VERSION" ]] || err "$host no reporta la versión $VERSION"
fi

if [[ -n $DRY_RUN ]]; then
	paso "DRY_RUN: se omiten tag, push y API. Se subirían:"
	ls -l dist
	exit 0
fi

if [[ -n $nuevo_tag ]]; then
	paso "tag $VERSION"
	git tag -a "$VERSION" -m "mkdocx $VERSION"
fi
git push -q origin "refs/tags/$VERSION"

gh_api() { # gh_api MÉTODO URL [args de curl...]
	local m=$1 u=$2
	shift 2
	curl -fsS -X "$m" "$u" \
		-H "Authorization: Bearer $GITHUB_TOKEN" \
		-H "Accept: application/vnd.github+json" \
		-H "X-GitHub-Api-Version: 2022-11-28" "$@"
}

paso "release en GitHub"
if rel=$(gh_api GET "$API/repos/$REPO/releases/tags/$VERSION" 2>/dev/null); then
	echo "    ya existe, se completa"
else
	rel=$(gh_api POST "$API/repos/$REPO/releases" \
		-d "$(jq -n --arg t "$VERSION" '{tag_name: $t, name: $t, generate_release_notes: true}')") ||
		err "no se pudo crear el release (¿token sin permiso Contents: write?)"
fi
id=$(jq -r .id <<<"$rel")
existentes=$(jq -r '.assets[].name' <<<"$rel")

for f in dist/*; do
	name=$(basename "$f")
	if grep -qxF "$name" <<<"$existentes"; then
		echo "    $name ya subido"
		continue
	fi
	echo "    subiendo $name"
	gh_api POST "$UPLOADS/repos/$REPO/releases/$id/assets?name=$name" \
		-H "Content-Type: application/octet-stream" --data-binary "@$f" >/dev/null ||
		err "falló la subida de $name; vuelve a ejecutar el script para reintentar"
done

paso "listo: $(jq -r .html_url <<<"$rel")"
