BINARY  := forge
MAIN    := ./cmd
WEB_DIR := ./web

.PHONY: build build-web test vet lint clean dev-web run

## build: compile the Go binary
build:
	go build -o $(BINARY) $(MAIN)

## build-web: build the frontend (tsc + vite)
build-web:
	cd $(WEB_DIR) && npm run build

## test: run all Go tests
test:
	go test ./...

## vet: run go vet
vet:
	go vet ./...

## lint: lint the frontend (eslint)
lint:
	cd $(WEB_DIR) && npm run lint

## dev-web: start the Vite dev server
dev-web:
	cd $(WEB_DIR) && npm run dev

## run: build then start the HTTP API server (addr defaults to :8080)
run: build
	./$(BINARY) serve

## clean: remove compiled binary and frontend dist
clean:
	rm -f $(BINARY)
	rm -rf $(WEB_DIR)/dist

help:
	@grep -E '^## ' Makefile | sed 's/## //'
