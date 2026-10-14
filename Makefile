# Builds ArenaSiegeSurvivorsLauncher.exe and publishes it as a GitHub release.
#
#   make build                   dist/ArenaSiegeSurvivorsLauncher.exe and dist/SHA256SUMS
#   make push VERSION=v0.1.0     build, tag v0.1.0, push the tag and upload the release
#
# Fyne needs cgo, so the Windows build uses MinGW-w64: the local one if installed, otherwise a
# Docker image with it (build/windows.Dockerfile). "make push" needs the GitHub CLI (gh), logged in.

APP      := ArenaSiegeSurvivorsLauncher
# The launcher was RogueArenaLauncher. Launchers from then (v0.0.3 on) update themselves from the
# release file of that name, so each release has a copy of the exe under it too.
OLD_APP  := RogueArenaLauncher
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DIST     := dist
EXE      := $(DIST)/$(APP).exe
OLD_EXE  := $(DIST)/$(OLD_APP).exe
ASSETS   := $(EXE) $(OLD_EXE) $(DIST)/SHA256SUMS

MINGW_CC := x86_64-w64-mingw32-gcc
LDFLAGS  := -s -w -H=windowsgui -X main.version=$(VERSION)
GO_BUILD := CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=$(MINGW_CC) \
	go build -trimpath -ldflags '$(LDFLAGS)' -o $(EXE) .

BUILDER_IMAGE := arena-siege-survivors-launcher-builder:go1.26
BUILD_CACHE   := $(HOME)/.cache/arena-siege-survivors-launcher-build

ifeq ($(shell command -v $(MINGW_CC) 2>/dev/null),)
DOCKER_BUILD ?= 1
endif

.PHONY: all build test vet builder push check-release clean

all: test build

test:
	go test ./internal/...

vet:
	go vet ./internal/...
	GOOS=windows go vet ./internal/...

build:
	@mkdir -p $(DIST)
ifeq ($(DOCKER_BUILD),1)
	@$(MAKE) --no-print-directory builder
	@mkdir -p $(BUILD_CACHE)
	docker run --rm -u $$(id -u):$$(id -g) \
		-e HOME=/tmp -e GOCACHE=/cache/go-build -e GOMODCACHE=/cache/mod \
		-v "$(CURDIR)":/src -v "$(BUILD_CACHE)":/cache -w /src \
		$(BUILDER_IMAGE) sh -c "git config --global --add safe.directory /src; $(GO_BUILD)"
else
	$(GO_BUILD)
endif
	cp $(EXE) $(OLD_EXE)
	cd $(DIST) && sha256sum $(APP).exe $(OLD_APP).exe > SHA256SUMS
	@echo "Built $(EXE) ($(VERSION))"

builder:
	docker build -q -t $(BUILDER_IMAGE) -f build/windows.Dockerfile build

# push releases the current commit as VERSION: it must be a vX.Y.Z tag name, the tracked files
# must be committed, and an existing tag must already point at this commit.
push: check-release build
	@if ! git rev-parse -q --verify "refs/tags/$(VERSION)" >/dev/null; then \
		echo "Tagging $(VERSION)"; \
		git tag -a "$(VERSION)" -m "$(APP) $(VERSION)"; \
	fi
	git push origin "refs/tags/$(VERSION)"
	@if gh release view "$(VERSION)" >/dev/null 2>&1; then \
		echo "Release $(VERSION) exists: replacing its files"; \
		gh release upload "$(VERSION)" $(ASSETS) --clobber; \
	else \
		gh release create "$(VERSION)" $(ASSETS) --verify-tag \
			--title "$(APP) $(VERSION)" --generate-notes; \
	fi

check-release:
	@command -v gh >/dev/null || { echo "make push needs the GitHub CLI: https://cli.github.com"; exit 1; }
	@gh auth status >/dev/null 2>&1 || { echo "Log in to GitHub first: gh auth login"; exit 1; }
	@echo "$(VERSION)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$$' || \
		{ echo "VERSION must be a release tag like v0.1.0 (got '$(VERSION)'): make push VERSION=v0.1.0"; exit 1; }
	@git diff-index --quiet HEAD -- || { echo "Commit your changes before releasing."; exit 1; }
	@if git rev-parse -q --verify "refs/tags/$(VERSION)" >/dev/null && \
		[ "$$(git rev-list -n1 "$(VERSION)")" != "$$(git rev-parse HEAD)" ]; then \
		echo "Tag $(VERSION) already exists on another commit."; exit 1; \
	fi

clean:
	rm -rf $(DIST)
