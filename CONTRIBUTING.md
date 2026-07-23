# Contributing to opencode-model-selector

## Quick Start

1. Fork and clone the repository
2. `go mod download`
3. `make test` to verify everything works
4. Create a branch, make changes, submit a PR

## Development

```bash
make build            # Build binary
make test             # Run all tests
make lint             # Run linter
make fmt              # Format code
make test-coverage    # Coverage report
```

## Issue-First Workflow

1. **Open an issue** before starting work — describe what and why
2. **Get approval** — wait for maintainer to add `status:approved` label
3. **Open a PR** linking the issue: `Closes #N` in the PR body
4. **Add a type label**: `type:bug`, `type:feature`, `type:docs`, `type:refactor`, `type:chore`

## Commit Format

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(tui): add fuzzy filter to model selection screen
fix(config): handle missing model field gracefully
refactor(opencode): extract provider grouping logic
docs(readme): update CLI flags table
```

## Code Style

- Follow existing patterns in the codebase
- Table-driven tests with `t.Run()` subtests
- Error wrapping: `fmt.Errorf("package: operation: %w", err)`
- Keep TUI state updates pure and testable
- Config mutations go through `Config.SetAgentField` / `Config.GetAgentField`

## Testing

- Target 70%+ coverage
- TUI tests: construct model, send messages, assert view/state
- Config tests: use temp files with known JSON fixtures
- Integration tests: exercise full parse → load → edit → save cycle
- CLI tests: call `Run()` with args, capture stdout/stderr

## Labels

| Category | Labels |
|----------|--------|
| Type (required on PR) | `type:bug`, `type:feature`, `type:docs`, `type:refactor`, `type:chore`, `type:breaking-change` |
| Status | `status:needs-review`, `status:approved`, `status:in-progress`, `status:blocked` |
| Priority | `priority:high`, `priority:medium`, `priority:low` |

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
