.PHONY: tidy build run test vet fmt clean

BINARY := bin/kh

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
