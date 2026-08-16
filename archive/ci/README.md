# CI workflow

`github-actions-ci.yml` is the GitHub Actions workflow for WinForge. It is
staged here rather than at `.github/workflows/ci.yml` because the automation
account that produced this branch does not hold the `workflows` permission and
GitHub rejects pushes that add or modify workflow files.

To enable it, a maintainer with write access should run:

```bash
mkdir -p .github/workflows
git mv ci/github-actions-ci.yml .github/workflows/ci.yml
git rm ci/README.md
git commit -m "Enable CI workflow"
```

## Why this matters

The preceding round of hardening was merged without a Go toolchain available,
so nothing was compiled or tested. That merge left `GOOS=windows go build` and
`go vet` failing, and the repository reported no CI checks to catch it.

The workflow covers exactly the gaps that allowed those defects through:

| Job | Purpose |
| --- | --- |
| `test` | `gofmt`, `go vet`, `go test`, and `go test -race` on Linux |
| `windows` | `go build`, `go vet`, and `go test` on a real `windows-latest` runner |
| `cross-compile` | `go build` and `go vet` for windows `amd64`, `386`, and `arm64` |
| `syntax` | `node --check` for JavaScript, JSON parsing, and whitespace checks |

The Windows-only code paths (COM, WinRT, WMI, SCM, registry) are guarded by
build tags, so a Linux-only pipeline never compiles them. The `windows` and
`cross-compile` jobs are what make those paths visible to CI.
