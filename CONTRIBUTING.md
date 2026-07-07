# Contributing

Thanks for helping improve `ctrwatch`. Keep changes small, tested, and easy to
review.

## Issues

Open an issue for bugs, feature ideas, runtime support requests, or confusing
documentation. Include:

- what you ran
- what you expected
- what happened instead
- runtime details, such as Docker, Podman, socket path, local or SSH
- logs or screenshots when they help

For new runtime support, describe the runtime API, how it exposes logs, inspect,
stats, and how it can be tested without requiring that runtime in normal unit
tests.

## Pull Requests

- Keep PRs focused on one feature, fix, or cleanup.
- Link the related issue when one exists.
- Update README or docs when behavior, commands, config, or runtime support
  changes.
- Add or update the smallest useful tests.
- Keep normal tests daemon-free, SSH-free, and terminal-free.
- Do not add dependencies unless the PR explains why the standard library or an
  existing dependency is not enough.

## Commit Style

Use conventional-style commit subjects:

```text
feat(tui): add inspect view
fix(runtime): handle empty stats response
docs(readme): document Podman socket setup
test(config): cover tag resolution
chore(deps): update Go module metadata
refactor(commands): share container resolution
```

Common types:

- `feat`: user-facing feature
- `fix`: bug fix
- `docs`: documentation-only change
- `test`: test-only change
- `chore`: maintenance, tooling, release metadata
- `refactor`: code change without behavior change

Use a short lowercase scope when useful, such as `tui`, `runtime`, `commands`,
`config`, `import`, or `docs`.

## Development Checks

Enable the local Git hooks:

```bash
git config core.hooksPath .githooks
```

The pre-commit hook runs `gofmt`, `go mod tidy -diff`, `go vet ./...`, and
`go test ./...`. CI also runs the race detector and coverage.

Run:

```bash
go test ./...
```

If the Go build cache is read-only:

```bash
GOCACHE=/tmp/ctrwatch-go-cache go test ./...
```

For coverage:

```bash
go test ./... -cover
```

Podman or other real-runtime checks should be opt-in integration tests, not part
of the default test suite.

## Project Direction

The roadmap lives in [docs/FEATURE_ROADMAP.md](docs/FEATURE_ROADMAP.md). The
current priority is making `ctrwatch` TUI-first while keeping direct commands
scriptable.
