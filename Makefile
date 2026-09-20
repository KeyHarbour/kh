.PHONY: tidy build run test test-coverage coverage-report vet fmt lint security clean install uninstall snapshot regression diagnostics release

BINARY := bin/kh
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
INSTALL ?= install
COVERAGE_DIR := coverage

.PHONY: build-cross release-local release-sync release-tag


 tidy:
	go mod tidy

lint:
	golangci-lint run ./...

security:
	govulncheck ./...
	gosec -no-fail ./...

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

build-cross:
	# cross-build for common OS/ARCH targets
	mkdir -p bin
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o bin/kh-darwin-arm64 ./cmd/kh
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o bin/kh-darwin-amd64 ./cmd/kh
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o bin/kh-linux-amd64 ./cmd/kh
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o bin/kh-linux-arm64 ./cmd/kh
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o bin/kh-windows-amd64.exe ./cmd/kh

release-local: build-cross
	# run goreleaser in snapshot mode to produce dist/ (requires goreleaser installed)
	goreleaser release --snapshot --clean

# Bump version, commit, and open a sync PR on KeyHarbour/kh.
# Usage: make release          (auto-detect bump from commits)
#        make release V=1.8.0  (explicit version)
#
# Does NOT tag. auto-tag.yml on the public repo creates vX.Y.Z once the sync PR
# is merged, which is the only point at which the version is actually released.
# Tagging here instead left strays behind whenever a sync PR was abandoned.
release:
	@./scripts/bump-version.sh $(if $(V),$(V),)
	@echo "Review CHANGELOG.md, then press Enter to commit and sync (Ctrl-C to abort)." && read _
	git add VERSION CHANGELOG.md
	git commit -m "chore: bump version to v$$(cat VERSION)"
	git push
	./scripts/sync-public.sh sync/v$$(cat VERSION)

# Tag the private repo at the version that has just been released publicly.
# Run this AFTER the sync PR is merged on KeyHarbour/kh — that merge is the
# moment the version actually ships. bump-version.sh derives the current
# version from this tag, so skipping it leaves the next release stuck on the
# previous version (the guard in bump-version.sh will catch that and say so).
release-tag:
	@v="$$(cat VERSION)"; \
	if git rev-parse "v$$v" >/dev/null 2>&1; then \
		echo "Tag v$$v already exists — nothing to do."; \
	else \
		echo "Tagging v$$v on $$(git rev-parse --abbrev-ref HEAD)."; \
		git tag "v$$v" && git push origin "v$$v"; \
	fi

# Sync an ALREADY-bumped VERSION to the public repo, without bumping or tagging.
# Use this when the version bump landed through a PR rather than through
# `make release` — running `make release` again would bump on top of it.
release-sync:
	@v="$$(cat VERSION)"; \
	echo "Syncing v$$v to KeyHarbour/kh (no bump, no tag)."; \
	./scripts/sync-public.sh "sync/v$$v"

# ---------------------------------------------------------------------------
# Integration tests (require KH_ENDPOINT and KH_TOKEN to be set)
# KH_SNAPSHOT_DIR defaults to ./integration-tests/testdata/snapshots
# ---------------------------------------------------------------------------
INTEG_FLAGS := -tags integration -v -count=1 -timeout 10m
SNAPSHOT_DIR ?= $(CURDIR)/integration-tests/testdata/snapshots

snapshot: build
	KH_TEST_MODE=snapshot \
	KH_SNAPSHOT_DIR=$(SNAPSHOT_DIR) \
	go test $(INTEG_FLAGS) ./integration-tests/... -run TestSnapshot

regression: build
	KH_TEST_MODE=regression \
	KH_SNAPSHOT_DIR=$(SNAPSHOT_DIR)/latest \
	go test $(INTEG_FLAGS) ./integration-tests/... -run TestRegression

diagnostics: build
	KH_TEST_MODE=diagnostics \
	go test $(INTEG_FLAGS) -timeout 3m ./integration-tests/... -run TestDiagnostics

# ---------------------------------------------------------------------------
# Install/uninstall the kh binary
install: build
	$(INSTALL) -d $(DESTDIR)$(BINDIR)
	$(INSTALL) -m 0755 $(BINARY) $(DESTDIR)$(BINDIR)/kh

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/kh
