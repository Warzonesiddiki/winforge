## What does this PR do?

<!-- A clear description of the change. Link related issues with Closes #NNN. -->

## Type of change

- [ ] Bug fix
- [ ] New feature / enhancement
- [ ] New tweak or catalog entry
- [ ] Documentation
- [ ] Refactor / maintenance
- [ ] CI / build

## Verification

<!-- Run the full battery and confirm each item. See CONTRIBUTING.md. -->

- [ ] `gofmt -l cmd internal embed.go` prints nothing
- [ ] `go vet ./cmd/... ./internal/... .` passes
- [ ] `go test ./cmd/... ./internal/... .` passes
- [ ] `go test -race ./cmd/... ./internal/... .` passes
- [ ] `GOOS=windows go build ./cmd/...` cross-compiles
- [ ] `python3 tools/catalog_parity.py` exits 0 (if catalog changed)
- [ ] `npm run typecheck && npm run lint && npm test` passes (if web code changed)
- [ ] `DATABASE_URL= npm run build` succeeds (if web code changed)

## Security considerations

<!-- If this changes the command allowlist, registry paths, plugin API, auth, or any system-mutating code path, explain the safety analysis here. -->

## Screenshots / output

<!-- For UI changes, paste a screenshot. For CLI changes, paste sample output. -->
