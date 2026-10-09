PROJECT_NAME = skillcheck
MAIN = ./cmd/skillcheck

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  = -s -w \
           -X main.version=$(VERSION) \
           -X main.commit=$(COMMIT) \
           -X main.date=$(DATE)

.PHONY: build install test test-npm lint clean

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(PROJECT_NAME) $(MAIN)

# Install skillcheck into the Go bin directory ($GOBIN, or $GOPATH/bin)
install:
	CGO_ENABLED=0 go install -ldflags="$(LDFLAGS)" $(MAIN)
	@bindir="$$(go env GOBIN)"; [ -n "$$bindir" ] || bindir="$$(go env GOPATH)/bin"; \
		echo "Installed $(PROJECT_NAME) to $$bindir"

test:
	go test ./... -count=1

# npm packaging, publish ordering, launcher exit-code and Slack message tests (needs node >= 18)
test-npm:
	node --test scripts/*.test.mjs

lint:
	golangci-lint run ./...

clean:
	rm -f $(PROJECT_NAME)

# Regenerate the embedded MQL schemas from the mql version pinned in go.mod
.PHONY: schemas
schemas:
	./scripts/gen-schemas.sh
