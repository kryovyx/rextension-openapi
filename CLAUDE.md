# rextension-openapi — OpenAPI document generation

`github.com/kryovyx/rextension-openapi`. Part of the REX framework: developed in a `go.work` workspace
alongside its siblings, released as its own module.

## Boundaries

Depends on `rextension` only.

Generates from the **frozen route table**. Reflection over handler function
values was removed deliberately: the route table already carries everything the
document needs, and reflecting over handlers produced documents that disagreed
with the routes actually served.

## Working here

- **Never `go build`.** Syntax-check with `go vet ./...`.
- **`go test -race ./...` always.**
- **Tests are per branch, not per coverage number.** Every branch of every
  function gets its own case; the README's coverage figure is recomputed from
  a measurement, never hand-edited.
- **No `replace` directives** in `go.mod`.
- **Commits:** [COMMIT-CONVENTIONS.md](COMMIT-CONVENTIONS.md) — gitmoji, a
  space, an imperative summary; no `type(scope)` prefix. At most one trailer,
  and no generated footers.
- `make check` here runs fmt, vet and race tests for this module alone.
- Default branch is `main`.
- **Contributing:** the framework is in alpha and external contributions open
  at `v1.0.0`. Packages carry a CONTRIBUTING.md with the rules that will
  apply; they are the same rules as these.

Design decisions are numbered (D…/O…/W…) and recorded in the workspace this
module is developed in, not in this repo. If a rule here looks arbitrary, it is
load-bearing — ask before removing it.
