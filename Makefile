BIN      := ctrwatch
GO       := go
LDFLAGS  := -s -w
FLAGS    := -trimpath
PKG      := .

.PHONY: all build test vet lint fmt clean run

all: build

build:
	$(GO) build $(FLAGS) -ldflags="$(LDFLAGS)" -o $(BIN) $(PKG)

build-static:
	CGO_ENABLED=0 $(GO) build $(FLAGS) -ldflags="$(LDFLAGS)" -o $(BIN) $(PKG)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -l -w .

lint:
	golangci-lint run ./...

clean:
	rm -f $(BIN)

run:
	$(GO) run $(PKG)

install:
	$(GO) install $(FLAGS) -ldflags="$(LDFLAGS)" $(PKG)

release:
	goreleaser release --clean
