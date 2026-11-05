BIN    := build/qrserver
DOCS   := ../docs
PUBLIC := web/public

all: build

run: build
	./$(BIN)

build: assets
	go build -o $(BIN) .

# La web app di docs/ viene servita così com'è su /crea/ (QR statici senza accesso)
# e la dashboard ne riusa renderer e stile: una sola copia del codice di disegno
assets:
	@rm -rf $(PUBLIC)
	@cp -r $(DOCS) $(PUBLIC)
	@rm -f $(PUBLIC)/.nojekyll

test: assets
	go test ./...

check: assets
	@test -z "$$(gofmt -l .)" || (gofmt -l .; echo "file da formattare con gofmt"; exit 1)
	go vet ./...
	go test -race ./...

clean:
	go clean
	rm -rf build $(PUBLIC)

.PHONY: all run build assets test check clean
