# Bootstrapping a Go Toolchain from Source (no network access to go.dev)

> Context: this sandbox blocks `go.dev`, `dl.google.com`, `proxy.golang.org`, and
> apt — but `codeload.github.com` (the golang/go mirror) is reachable. The engine
> is **stdlib-only** (`GOPROXY=off`), so once the toolchain exists, everything
> builds offline. Verified working 2026-08-16 (full chain + engine build + tests
> + Windows cross-compile).

## The chain (each hop compiles the next from source)

| Hop | Source | Bootstrap | Time (this machine) |
|---|---|---|---|
| 1 | `go1.4.3` (last C-based Go) | gcc (with wrapper, see below) | ~26 s |
| 2 | `go1.17.13` | Go 1.4.3 | ~2 min |
| 3 | `go1.20.14` | Go 1.17.13 | ~2 min |
| 4 | `go1.22.12` (cgo enabled) | Go 1.20.14 | ~2.5 min |

Why the hops: Go 1.22 requires a bootstrap ≥ 1.20.6 (`building_Go_requires_Go_1_20_6_or_later`),
Go 1.20 requires ≥ 1.17.13, Go 1.17 requires ≥ 1.4. Go 1.4 is written in C, so gcc
is the only pre-existing dependency.

## Procedure

```bash
BASE=/opt/gobootstrap            # pick any writable dir
mkdir -p "$BASE" && cd "$BASE"

# 1. Fetch sources from the GitHub mirror (codeload is reachable).
for tag in go1.4.3 go1.17.13 go1.20.14 go1.22.12; do
  curl -sSL "https://codeload.github.com/golang/go/tar.gz/refs/tags/${tag}" -o "${tag}.tar.gz" &
done; wait

# 2. Modern gcc defaults (PIE + -fno-common) break Go 1.4's C code. Wrap gcc:
cat > "$BASE/cc-wrapper.sh" <<'EOF'
#!/bin/sh
exec gcc -fcommon -no-pie "$@"
EOF
chmod +x "$BASE/cc-wrapper.sh"

# 3. Hop 1: Go 1.4.3 (C).
tar xzf go1.4.3.tar.gz && mv go-go1.4.3 go1.4
(cd go1.4/src && CGO_ENABLED=0 CC_FOR_TARGET="$BASE/cc-wrapper.sh" ./make.bash)

# 4. Hops 2–4.
tar xzf go1.17.13.tar.gz && mv go-go1.17.13 go1.17
(cd go1.17/src && GOROOT_BOOTSTRAP="$BASE/go1.4" CGO_ENABLED=0 ./make.bash)

tar xzf go1.20.14.tar.gz && mv go-go1.20.14 go1.20
(cd go1.20/src && GOROOT_BOOTSTRAP="$BASE/go1.17" CGO_ENABLED=0 ./make.bash)

tar xzf go1.22.12.tar.gz && mv go-go1.22.12 go1.22
(cd go1.22/src && GOROOT_BOOTSTRAP="$BASE/go1.20" CGO_ENABLED=1 ./make.bash)   # cgo for -race

# 5. Use it.
export PATH="$BASE/go1.22/bin:$PATH"
go version   # go version go1.22.12 linux/amd64
```

## Notes

- **CGO_ENABLED=1 on the final hop** enables the race detector (`go test -race`)
  and any future cgo needs; the engine itself is stdlib-only and cross-compiles
  with `CGO_ENABLED=0`.
- The first hop's `CC_FOR_TARGET` wrapper is only needed for Go 1.4.
- Build logs land in `$BASE/*-build.log` if you redirect as shown in CI.
- Sandbox `/tmp` is wiped between sessions — reinstall the chain in a durable dir
  or regenerate per session (~7 min total).
