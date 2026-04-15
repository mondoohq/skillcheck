PROJECT_NAME = skillcheck
MAIN = ./cmd/skillcheck

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  = -s -w \
           -X main.version=$(VERSION) \
           -X main.commit=$(COMMIT) \
           -X main.date=$(DATE)

.PHONY: build test lint clean

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(PROJECT_NAME) $(MAIN)

test:
	go test ./... -count=1

lint:
	golangci-lint run ./...

clean:
	rm -f $(PROJECT_NAME)

# Update embedded MQL schema files from the local mql checkout
.PHONY: schemas
schemas:
	@echo "Copying schemas from ../mql ..."
	cp ../mql/providers/os/resources/os.resources.json internal/engine/schemas/os.resources.json
	cp ../mql/providers/core/resources/core.resources.json internal/engine/schemas/core.resources.json
