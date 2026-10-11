#!/bin/sh
# Instala el binario de mkdocx desde los releases de GitHub (Linux y macOS).
# Verifica el SHA256 contra SHA256SUMS antes de instalar.
#
# Variables opcionales:
#   MKDOCX_VERSION   etiqueta a instalar (por defecto: latest)
#   MKDOCX_BIN_DIR   destino (por defecto: ~/.local/bin)
#   MKDOCX_BASE_URL  URL base de descarga (para pruebas o mirrors)
set -eu

REPO=bioodev/mkdocx
VERSION=${MKDOCX_VERSION:-latest}
BIN_DIR=${MKDOCX_BIN_DIR:-$HOME/.local/bin}

if [ -n "${MKDOCX_BASE_URL:-}" ]; then
	BASE=$MKDOCX_BASE_URL
elif [ "$VERSION" = latest ]; then
	BASE="https://github.com/$REPO/releases/latest/download"
else
	BASE="https://github.com/$REPO/releases/download/$VERSION"
fi

err() { echo "install.sh: $*" >&2; exit 1; }

case $(uname -s) in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) err "sistema no soportado: $(uname -s) (en Windows descarga mkdocx-windows-amd64.exe desde https://github.com/$REPO/releases/latest)" ;;
esac
case $(uname -m) in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) err "arquitectura no soportada: $(uname -m)" ;;
esac
asset="mkdocx-$os-$arch"

if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -q -O "$2" "$1"; }
else
	err "se necesita curl o wget"
fi
if command -v sha256sum >/dev/null 2>&1; then
	sha() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
	sha() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
	err "se necesita sha256sum o shasum"
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fetch "$BASE/$asset" "$tmp/$asset" || err "no se pudo descargar $BASE/$asset"
fetch "$BASE/SHA256SUMS" "$tmp/SHA256SUMS" || err "no se pudo descargar $BASE/SHA256SUMS"

want=$(awk -v f="$asset" '$2 == f || $2 == "*" f { print $1 }' "$tmp/SHA256SUMS")
[ -n "$want" ] || err "$asset no figura en SHA256SUMS"
[ "$(sha "$tmp/$asset")" = "$want" ] || err "SHA256 no coincide para $asset"

mkdir -p "$BIN_DIR"
chmod 755 "$tmp/$asset"
mv "$tmp/$asset" "$BIN_DIR/mkdocx"

echo "instalado: $BIN_DIR/mkdocx ($("$BIN_DIR/mkdocx" --version))"
case ":$PATH:" in
	*":$BIN_DIR:"*) ;;
	*) echo "aviso: $BIN_DIR no está en PATH; usa $BIN_DIR/mkdocx o añádelo al PATH" >&2 ;;
esac
