# Copilot instructions for socks5lb

socks5lb is a Go library and CLI that load-balances TCP connections across healthy
upstream SOCKS5 proxies. See the [README](../README.md) for configuration, management
API examples, and deployment.

## Build, test, and lint

Run commands from the repository root. Use the mise-managed Go version pinned in
[mise.toml](../mise.toml) and [go.mod](../go.mod), currently Go 1.26.8.

```sh
mise install
mise exec -- make build      # Produces ./socks5lb with version/build metadata
mise exec -- make check      # Formatting check, go vet, tests, race detector
mise exec -- make test       # go test ./...
mise exec -- make race       # CGO_ENABLED=1 go test -race ./...
mise exec -- make vet        # go vet ./...
mise exec -- make fmt-check  # Check gofmt without modifying files
mise exec -- make fmt        # Apply gofmt
```

Run a single library test, CLI test, or subtest:

```sh
mise exec -- go test . -run '^TestPool_NextCheck$' -count=1
mise exec -- go test ./cmd/socks5lb -run '^TestNewConfig$' -count=1
mise exec -- go test ./cmd/socks5lb -run '^TestNewConfig$/^valid$' -count=1
```

[Taskfile.yml](../Taskfile.yml) provides equivalent named targets, for example
`mise exec -- task check` and `mise exec -- task build`. Container builds use
`docker-compose build`.

The race detector needs a C compiler; the race targets override `CGO_ENABLED=0`
for that command. If an inherited `GOROOT` selects another Go installation, use
`env -u GOROOT mise exec -- make check` rather than changing the global toolchain.

Build targets inject `Version`, `BuildCommit`, and `BuildDate` into
[version.go](../version.go). When updating Go, keep the pins in
[go.mod](../go.mod), [mise.toml](../mise.toml), [Dockerfile](../Dockerfile),
[Drone](../.drone.yml), and [GitLab CI](../.gitlab-ci.yml) aligned with the README.
[GitHub Actions](workflows/go.yml) reads the version from go.mod and runs
`make check` followed by `make build`.

## Architecture

- The root `socks5lb` package is the reusable library.
  [The CLI](../cmd/socks5lb/main.go) reads YAML with `-c` (default
  `/etc/socks5lb.yml`), validates `server.socks5.addr`, and runs a `go-svc`
  [program](../cmd/socks5lb/program.go). Its initialization constructs the pool
  and server; its context signals server failures to the service runner.
- The normal data path is [the SOCKS5 listener](../socks5.go) -> `Pool.Next()`
  -> TCP dial to the selected backend -> `Server.Transport`. It forwards the
  client's SOCKS5 negotiation unchanged, rather than terminating it or injecting
  configured backend credentials. Clients authenticate with the selected upstream.
- [Server](../server.go) schedules [pool](../pool.go) health checks using
  `CHECK_TIME_INTERVAL` (seconds, default 60). [Backend](../backend.go) performs
  HEAD requests through an authenticated SOCKS5 connection; HTTP 2xx/3xx is healthy
  and redirects are not followed. Without `check_url`, health follows
  `initial_alive`. Health-check timeout is in seconds and defaults to 10.
- [The HTTP management router](../http.go) belongs to each server and updates
  that server's live, in-memory pool. HTTP is optional via `server.http.addr`;
  it has no built-in authentication and must be bound to a trusted interface or
  protected by an authenticated reverse proxy.
- [Linux transparent proxy support](../redirect_linux.go) is a deprecated,
  explicit library API; neither the CLI nor `Server.Start` calls `ListenTProxy`.
  Setting `server.tproxy.addr` does not enable it. `SELECT_TIME_INTERVAL`
  (seconds, default 300) affects only this path, not normal per-connection
  round-robin selection. Preserve the Linux/non-Linux build-tag split with
  [redirect.go](../redirect.go).

## Key conventions

- Server lifecycle is per instance: keep pools, routers, listeners, and connection
  ownership out of global singletons. `Start` blocks and returns listener errors.
  `Stop` is idempotent, including before startup; it cancels checks, closes
  listeners/connections, waits for proxy workers, and allows five seconds for
  HTTP shutdown. Stopped instances cannot restart. New connection paths must
  participate in the server context and `trackConnection`/`releaseConnection`
  bookkeeping in [server.go](../server.go).
- Pools are keyed by exact backend address. `All` returns an address-sorted
  snapshot, and `Next` selects round-robin from the healthy snapshot. Use
  `Alive`/`SetAlive` for atomic health access, and keep network health-check I/O
  outside the pool mutex.
- [Configuration](../config.go) and API binding share the
  [Backend type](../backend.go). Preserve YAML/JSON field names and complete
  backend values, including credentials. Apply `CheckConfig.InitialAlive`
  before inserting configured or dynamically added backends.
- Preserve `/api/add` batch atomicity through `Pool.addBackends`: invalid/empty
  input returns HTTP 400; existing addresses or duplicates within a batch return
  HTTP 503 without modifying the pool. Success is plain text,
  `N backend(s) added`. Replacing a backend requires deleting it first.
- SOCKS5 connection timeouts bound dialing and negotiation, not established
  tunnel lifetime. Keep cancellation wired through negotiation and preserve
  TCP `CloseWrite` support: finishing one copy direction must still allow the
  reverse response to drain. Reuse `Transport` rather than adding another
  bidirectional-copy implementation.
- Tests use package-local access, `testify`, and local listeners/`httptest`,
  without public-network dependencies. Reuse `tcpConnPair`, `receiveError`, and
  `networkTestTimeout` from [server_test.go](../server_test.go), and
  `serveTestSOCKS5` from [backend_test.go](../backend_test.go). Health-only tests
  can inject `Backend.checkHTTP`. HTTP tests call `setupRouter` before exercising
  `Engine().ServeHTTP`, as in [http_test.go](../http_test.go); `NewServer` alone
  does not initialize the router. Privileged transparent-routing integration
  needs a separate Linux environment with the required routing rules.
