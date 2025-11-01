.PHONY: tidy build run test test-coverage coverage-report vet fmt clean install uninstall

BINARY := bin/kh
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
INSTALL ?= install
COVERAGE_DIR := coverage

 tidy:
	go mod tidy

build:
	mkdir -p bin
	go build -o $(BINARY) ./cmd/kh

run: build
	$(BINARY) $(args)

test:
	go test ./...

test-coverage:
	mkdir -p $(COVERAGE_DIR)
	go test -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	go tool cover -func=$(COVERAGE_DIR)/coverage.out

coverage-report:
	mkdir -p $(COVERAGE_DIR)
	go test -coverprofile=$(COVERAGE_DIR)/coverage.out ./... || true
	go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	go tool cover -func=$(COVERAGE_DIR)/coverage.out > $(COVERAGE_DIR)/coverage.txt

vet:
	go vet ./...

fmt:
	go fmt ./...

clean:
	rm -rf bin $(COVERAGE_DIR)

# Install/uninstall the kh binary
install: build
	$(INSTALL) -d $(DESTDIR)$(BINDIR)
	$(INSTALL) -m 0755 $(BINARY) $(DESTDIR)$(BINDIR)/kh

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/kh
