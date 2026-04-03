.PHONY: build test clean cross install changelog release release-patch release-minor release-major

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

build:
	docker compose run --rm dev go build -ldflags="$(LDFLAGS)" -o bin/tally .

test:
	docker compose run --rm dev go test ./... -v

coverage:
	docker compose run --rm dev go test -coverprofile=coverage.out ./...
	docker compose run --rm dev go tool cover -func=coverage.out

clean:
	rm -rf bin/

UNAME_S := $(shell uname -s)
ifeq ($(findstring MINGW,$(UNAME_S)),MINGW)
  INSTALL_DIR := $(shell echo "$$LOCALAPPDATA/Programs/tally" | sed 's|\\|/|g')
  EXT := .exe
else
  INSTALL_DIR := $(HOME)/.local/bin
  EXT :=
endif

install: build
	mkdir -p "$(INSTALL_DIR)"
	cp bin/tally$(EXT) "$(INSTALL_DIR)/tally$(EXT)"
	@echo "installed to $(INSTALL_DIR)/tally$(EXT)"

changelog:
	docker compose run --rm dev git-cliff --output CHANGELOG.md

release: changelog
	@NEXT=$(NEXT); \
	if [ -z "$$NEXT" ]; then echo "usage: make release NEXT=v1.2.3"; exit 1; fi; \
	git add CHANGELOG.md && \
	git commit -m "chore: update changelog for $$NEXT" && \
	git tag $$NEXT && \
	{ git push origin HEAD $$NEXT && echo "released $$NEXT"; } || { git tag -d $$NEXT; git reset --soft HEAD~1; echo "push failed — tag and commit rolled back"; exit 1; }

release-patch:
	$(eval NEXT := $(shell docker compose run --rm dev sh -c "git describe --tags --abbrev=0 2>/dev/null | awk -F. '{printf \"%s.%s.%d\", \$$1, \$$2, \$$3+1}'"))
	$(MAKE) release NEXT=$(NEXT)

release-minor:
	$(eval NEXT := $(shell docker compose run --rm dev sh -c "git describe --tags --abbrev=0 2>/dev/null | awk -F. '{printf \"%s.%d.0\", \$$1, \$$2+1}'"))
	$(MAKE) release NEXT=$(NEXT)

release-major:
	$(eval NEXT := $(shell docker compose run --rm dev sh -c "git describe --tags --abbrev=0 2>/dev/null | awk -F. '{printf \"v%d.0.0\", substr(\$$1,2)+1}'"))
	$(MAKE) release NEXT=$(NEXT)

cross:
	docker compose run --rm dev sh -c "\
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='$(LDFLAGS)' -o bin/tally-linux-amd64 . && \
		CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags='$(LDFLAGS)' -o bin/tally-darwin-amd64 . && \
		CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags='$(LDFLAGS)' -o bin/tally-darwin-arm64 . && \
		CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags='$(LDFLAGS)' -o bin/tally-windows-amd64.exe ."
