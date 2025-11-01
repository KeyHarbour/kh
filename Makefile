.PHONY: tidy build run test vet fmt clean install uninstall

BINARY := bin/kh
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
INSTALL ?= install

 tidy:
	go mod tidy

build:
	mkdir -p bin
	go build -o $(BINARY) ./cmd/kh

run: build
	$(BINARY) $(args)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

clean:
	rm -rf bin

# Install/uninstall the kh binary
install: build
	$(INSTALL) -d $(DESTDIR)$(BINDIR)
	$(INSTALL) -m 0755 $(BINARY) $(DESTDIR)$(BINDIR)/kh

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/kh
