# Contributing to lazyredis

Thanks for your interest in contributing.

## Branch model

| Branch | Purpose |
|--------|---------|
| `main` | Stable releases only |
| `develop` | Integration branch for ongoing work |
| `feature/*` | New features (from `develop`) |
| `fix/*` | Bug fixes (from `develop`) |
| `release/*` | Release preparation (from `develop`) |

## Workflow

1. Fork the repository and clone it locally.
2. Create a branch from `develop`:
   ```bash
   git checkout develop
   git pull origin develop
   git checkout -b feature/my-change
   ```
3. Make your changes and add tests when behavior changes.
4. Run checks locally:
   ```bash
   go test -short ./...
   go build -o lazyredis ./cmd/lazyredis
   ```
5. Open a pull request against **`develop`**.
6. Wait for CI and review. Address feedback when needed.
7. Maintainers merge to `develop`. Releases are promoted from `develop` to `main`.

## Pull requests

- Keep PRs focused and small when possible.
- Describe **what** changed and **why**.
- Link related issues when applicable.
- Do not commit secrets, credentials, or environment-specific hostnames.

## Integration tests

Optional docker-based tests:

```bash
./test/up.sh
go test -tags=integration ./internal/store -run TestIntegration -count=1
./test/down.sh
```

## Code style

- Match existing Go style in the repository.
- Run `go fmt ./...` before submitting.
- Avoid unrelated refactors in the same PR.

## Questions

Open a [GitHub issue](https://github.com/bloodynite/lazyredis/issues) for bugs, features, or questions.
