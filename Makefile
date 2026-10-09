VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
MAX_SIZE_BYTES := 10485760
export CGO_ENABLED := 0

.PHONY: build test size release
build:
	go build -ldflags '$(LDFLAGS)' -o bin/mkdocx ./cmd/mkdocx

test:
	go vet ./... && go test ./...

# Falla si el binario supera 10 MB (criterio de "liviano").
size: build
	@s=$$(wc -c < bin/mkdocx); echo "bin/mkdocx: $$s bytes"; \
	if [ $$s -gt $(MAX_SIZE_BYTES) ]; then echo "excede 10 MB"; exit 1; fi

release:
	rm -rf dist && mkdir dist
	for t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do \
	  os=$${t%/*}; arch=$${t#*/}; ext=""; [ $$os = windows ] && ext=".exe"; \
	  GOOS=$$os GOARCH=$$arch go build -ldflags '$(LDFLAGS)' -o dist/mkdocx-$$os-$$arch$$ext ./cmd/mkdocx || exit 1; \
	done
	cd dist && sha256sum * > SHA256SUMS
