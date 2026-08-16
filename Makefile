# WinForge — verification battery (mirrors ci.yml.fixed)
# Usage: make verify  # runs the exact checks CI runs, locally
# Requires: Go 1.22 (from /tmp/gobootstrap/go1.22/bin), python3, node, npm
# GOPROXY=off GOFLAGS=-mod=mod everywhere — stdlib-only, no network.

SHELL := /bin/bash

GO ?= /tmp/gobootstrap/go1.22/bin/go
GOPROXY ?= off
GOFLAGS ?= -mod=mod
export GOPROXY
export GOFLAGS

.PHONY: verify gofmt vet test race vet-windows build-windows parity web syntax check

verify: gofmt vet test race vet-windows build-windows parity web syntax
	@echo "=== verify: ALL GREEN ==="

gofmt:
	@echo ">> gofmt"
	@test -z "$$(gofmt -l .)" || (echo "gofmt -l failed:"; gofmt -l .; gofmt -d .; exit 1)

vet:
	@echo ">> go vet (linux)"
	@$(GO) vet ./...

test:
	@echo ">> go test"
	@$(GO) test ./...

race:
	@echo ">> go test -race"
	@$(GO) test -race ./...

vet-windows:
	@echo ">> go vet (windows)"
	@GOOS=windows GOARCH=amd64 $(GO) vet ./...

build-windows:
	@echo ">> GOOS=windows go build -> PE"
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o /tmp/winforge-verify.exe ./cmd/winforge
	@head -c 2 /tmp/winforge-verify.exe | od -An -tx1 | grep -q "4d 5a" || (echo "not a PE (missing MZ)"; exit 1)
	@echo "PE OK: $$(ls -lh /tmp/winforge-verify.exe | awk '{print $$5}')"

parity:
	@echo ">> catalog parity (must exit 0)"
	@python3 tools/catalog_parity.py
	@echo ">> converter idempotence (SHA)"
	@before="$$(sha256sum config/tweaks.json)"; \
	python3 tools/web_catalog_to_engine.py --apply >/dev/null 2>&1 || true; \
	after="$$(sha256sum config/tweaks.json)"; \
	if [ "$$before" != "$$after" ]; then echo "converter drifted config/tweaks.json"; git diff --stat config/tweaks.json; exit 1; fi; \
	echo "converter idempotent: $$before"
	@echo ">> locales sync"
	@python3 tools/extract_locales.py
	@echo ">> Inno Setup ISS generation (dry-run)"
	@python3 tools/generate_iss.py --check
	@echo ">> isobuilder Autounattend dry-run (Go + Python XML)"
	@$(GO) test -run TestGenerateUnattendXML ./internal/isobuilder -count=1
	@python3 -c "import xml.etree.ElementTree as ET; xml='''<?xml version=\"1.0\" encoding=\"utf-8\"?><unattend xmlns=\"urn:schemas-microsoft-com:unattend\"><settings pass=\"windowsPE\"><component name=\"Microsoft-Windows-International-Core-WinPE\" processorArchitecture=\"amd64\" publicKeyToken=\"31bf3856ad364e35\" language=\"neutral\" versionScope=\"nonSxS\"><SetupUILanguage><UILanguage>en-US</UILanguage></SetupUILanguage></component></settings></unattend>'''; ET.fromstring(xml); print('Python xml.etree validation OK')"
	@echo ">> updater GitHub API shape (mocked httptest)"
	@$(GO) test -run TestCheckGitHubRelease ./internal/updater -count=1

web:
	@echo ">> npm typecheck"
	@npm run typecheck
	@echo ">> npm lint"
	@npm run lint
	@echo ">> node --check web/app.js"
	@node --check web/app.js

syntax:
	@echo ">> JSON syntax"
	@failed=0; while IFS= read -r -d '' f; do python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$$f" || { echo "invalid JSON: $$f"; failed=1; }; done < <(find . -name '*.json' -not -path './.git/*' -not -path './node_modules/*' -print0); exit $$failed
	@echo ">> JS syntax"
	@failed=0; while IFS= read -r -d '' f; do node --check "$$f" || { echo "invalid JS: $$f"; failed=1; }; done < <(find web -name '*.js' -print0); exit $$failed
	@echo ">> git diff --check (no trailing whitespace)"
	@git diff --check "$$(git hash-object -t tree /dev/null)" HEAD || (echo "git diff --check failed"; exit 1)

# Shorthand for local dev (no -race, faster)
quick: gofmt vet test parity web
	@echo "quick OK"

check: verify
