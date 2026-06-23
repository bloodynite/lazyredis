# AGENTS.md

Conventions for AI agents working in this repo.

## Code style

- **No comments in code.** Names should be self-explanatory. If a function
  needs explanation, the explanation belongs in the commit message, not
  in the source. Exception: short single-line comments above exported
  identifiers when the type signature is not enough.
- One behavior per commit (conventional commits).
- Tests next to code (`<file>_test.go` in the same package).
- Keep changes under ~300 lines per commit.
