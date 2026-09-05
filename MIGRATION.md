# rextension-openapi — migration to v0.3.0

**v0.2.1 → v0.3.0** · Go 1.27 · still pre-1.0 (**alpha**)

This is the `rextension-openapi` chapter of the REX v0.3.0 upgrade, written to stand alone:
you need nothing else to upgrade this module. If you use other modules of the
framework, each has its own guide — they are listed at the bottom.

---

## Before you start

- **Go 1.27 is a hard floor**, not a courtesy bump. The framework's own source
  uses `encoding/json/v2` and 1.27 struct-literal syntax. There is no build path
  on 1.26.
- **The modules depend on each other, so a partial upgrade does not compile.**
  Bump them in dependency order as a single change, using the sequence below,
  even if only one of them is the reason you are here.
- **Still alpha.** These are breaking changes and the versions stay pre-1.0
  deliberately. Pin exact versions; there is no compatibility promise yet.

---

## Upgrade in this order

```sh
# 1. the contract everything else compiles against
go get github.com/kryovyx/rextension@v0.3.0

# 2. the container, then the framework
go get github.com/kryovyx/dix@v0.2.0
go get github.com/kryovyx/rex@v0.3.0

# 3. the extensions you actually use
go get github.com/kryovyx/rextension-security@v0.6.0
go get github.com/kryovyx/rextension-validation@v0.3.0
go get github.com/kryovyx/rextension-openapi@v0.3.0
go get github.com/kryovyx/rextension-health@v0.3.0
go get github.com/kryovyx/rextension-metric@v0.3.0
go get github.com/kryovyx/rextension-swagger@v0.3.0

# 4. new, and optional
go get github.com/kryovyx/rextension-cors@v0.1.0
go get github.com/kryovyx/rextension-ratelimit@v0.1.0

# 5. the WebSocket side, if you want it. Additive: skipping this
#    changes nothing about the HTTP application.
go get github.com/kryovyx/rextension-wsx@v0.1.0

go mod tidy && go build ./...
```

`corex` is deliberately absent from that list. It is new in this release and
arrives as a dependency of `rextension`, which re-exports all of it as type
aliases; `go mod tidy` writes the require line for you. You name it explicitly
only if you call `corex.ConfigureProblems`.

> **If you use a `go.work` file**
>
> A workspace builds every module against sibling *source*, which hides exactly
> this kind of version skew — a module can be broken against its declared
> dependencies and still build for you. Verify with
> `GOWORK=off go build ./...` before you trust a green build.

---

## The one change behind this release

Rex now decides its routing and middleware once, at startup, instead of per
request. `New()` and the extension hooks **declare**; `Run()` **builds** — it
composes every middleware chain, builds the route tables, freezes them, and
only then binds listeners. Nothing is mutated after that.

Two consequences reach the whole framework:

- **Register routes and middleware from `OnInitialize` or `OnStart`, never from
  `OnReady`.** By `OnReady` the listeners are bound and the table is frozen, so
  registration now returns `ErrRouterFrozen` rather than mutating a trie that
  in-flight requests are reading.
- **Several things that used to fail silently now fail at startup** — an
  unknown security scheme name, a route requiring roles from a scheme that
  cannot enforce them, middleware registered for a router nothing creates, two
  routes collapsing to one trie key. Expect a boot failure or two on the first
  run. Each one is naming a bug that was already there.

The full account, with the new startup-time route validation that comes with
it, is in [`rex`'s guide](https://github.com/kryovyx/rex/blob/main/MIGRATION.md).

---

## What changed in `rextension-openapi`

### Only the default router is documented

Every router used to be collected, so a route on an internal-only listener was
published in the document served on the public one. The default is now the
default router alone: a document that omits a route can be widened by
configuration, whereas a document that published a private route has already
published it.

```go
openapi.WithIncludeRouters("default", "api")  // or "*" for all
openapi.WithExcludeRouters("internal")        // applied after include
openapi.WithServeOnRouter("internal")         // where the spec is served
```

**If routes disappeared from your document**, they were on a non-default router.
Add it to `WithIncludeRouters` — and check first whether you meant to publish
them.

### The document is generated at startup

It was built lazily, on the first request to the spec path, from routes gathered
off an *asynchronous* event while generation ran synchronously. The spec could
come back with any subset of your routes — including none — depending purely on
goroutine scheduling. In a small application the events usually drained in time,
so it looked fine.

Generation now happens once, from the complete route table, via
`RouteValidator`. Two consequences: **a generation failure is now a startup
error**, not a 500 on the spec path; and the document is written once from one
goroutine rather than by whichever request arrived first.

### Reflection replaced with interfaces

Schema extraction was four `reflect.MethodByName` calls ending in an unchecked
type assertion that panicked inside document generation if `Types()` returned a
differently-typed slice. It reads the `rextension` interfaces directly now.
`BodySchemaAccessor` and `SchemaKind` are aliases of the `rextension` types.

---

## Verification

- [ ] `GOWORK=off go build ./...` passes — a workspace hides version skew, so
      this is the check that matters.
- [ ] `go test -race ./...` passes. The container and event bus were both
      unsynchronised before; if your tests never ran with `-race`, run them now.
- [ ] The application **starts**. Startup failures are the point of this
      release; each one names a pre-existing bug.

---

*Part of the REX v0.3.0 upgrade. The other guides:*

- [`rextension`](https://github.com/kryovyx/rextension/blob/main/MIGRATION.md) — v0.2.1 → **v0.3.0**
- [`rex`](https://github.com/kryovyx/rex/blob/main/MIGRATION.md) — v0.2.1 → **v0.3.0**
- [`dix`](https://github.com/kryovyx/dix/blob/main/MIGRATION.md) — v0.1.0 → **v0.2.0**
- [`rextension-security`](https://github.com/kryovyx/rextension-security/blob/main/MIGRATION.md) — v0.5.0 → **v0.6.0**
- [`rextension-validation`](https://github.com/kryovyx/rextension-validation/blob/main/MIGRATION.md) — v0.2.0 → **v0.3.0**
- [`rextension-health`](https://github.com/kryovyx/rextension-health/blob/main/MIGRATION.md) — v0.2.1 → **v0.3.0**
- [`rextension-metric`](https://github.com/kryovyx/rextension-metric/blob/main/MIGRATION.md) — v0.2.1 → **v0.3.0**
- [`rextension-swagger`](https://github.com/kryovyx/rextension-swagger/blob/main/MIGRATION.md) — v0.2.1 → **v0.3.0**
- [`corex`](https://github.com/kryovyx/corex/blob/main/MIGRATION.md) — new in this release, **v0.1.0**
- [`rextension-cors`](https://github.com/kryovyx/rextension-cors/blob/main/MIGRATION.md) — new in this release, **v0.1.0**
- [`rextension-ratelimit`](https://github.com/kryovyx/rextension-ratelimit/blob/main/MIGRATION.md) — new in this release, **v0.1.0**
- [`wsxtension`](https://github.com/kryovyx/wsxtension/blob/main/MIGRATION.md) — new in this release, **v0.1.0**
- [`wsx`](https://github.com/kryovyx/wsx/blob/main/MIGRATION.md) — new in this release, **v0.1.0**
- [`wsxtension-asyncapi`](https://github.com/kryovyx/wsxtension-asyncapi/blob/main/MIGRATION.md) — new in this release, **v0.1.0**
- [`wsxtension-lens`](https://github.com/kryovyx/wsxtension-lens/blob/main/MIGRATION.md) — new in this release, **v0.1.0**
- [`rextension-wsx`](https://github.com/kryovyx/rextension-wsx/blob/main/MIGRATION.md) — new in this release, **v0.1.0**

*Every one of them stands alone; read only the ones for modules you use.*
