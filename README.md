# socks5lb - Simple SOCKS5 Proxy Load Balancer

![socks5lb](./asserts/socks5lb.png)

Sometimes SOCKS5 proxies become unreachable due to network fluctuations or routing changes, requiring manual node switching which can be tedious and time-consuming.

This tool solves that problem by acting as a front-end load balancer for SOCKS5 proxies, automatically routing traffic through verified, healthy proxy nodes.

The library also retains a deprecated Linux transparent proxy API with SOCKS5 protocol conversion. It is not started by the CLI or `Server.Start`.

## Key Features

- **Load Balancing**: Round-robin distribution across SOCKS5 proxies with automatic health checks
- **Legacy Transparent Proxy API**: Linux [TPROXY](https://www.kernel.org/doc/Documentation/networking/tproxy.txt) support with SOCKS5 protocol conversion; deprecated and opt-in for library callers
- **Cross-Platform**: Written in Go for easy deployment across platforms (including routers)

## Changelog

- `2025-10-07` Refactored codebase to update copyright information, improve health check mechanisms, and enhance concurrency handling with atomic operations and mutexes. Bump Go version to 1.25 and optimize HTTP server configurations.
- `2022-07-16` Fixed connection performance issues, added HTTP management interface
- `2022-07-06` Completed Linux transparent gateway functionality
- `2022-06-20` Initial release with core features

## Building

The Go toolchain is managed by [mise](https://mise.jdx.dev/). The version in
`mise.toml` matches `go.mod` (Go 1.26.8); use the project toolchain for development:

```bash
mise install
mise exec -- make build
```

This produces `./socks5lb`. Docker Compose is also supported:

```bash
docker-compose build
```

## Configuration

Here's a basic configuration example with three SOCKS5 proxies exposed on local port 1080:

```yaml
server:
  http:
    addr: ":8080"
  socks5:
    addr: ":1080"
backends:
  - addr: 192.168.100.254:1086
    check_config:
      check_url: https://www.google.com/robots.txt
      initial_alive: true
      timeout: 3
  - addr: 10.1.0.254:1086
    check_config:
      check_url: https://www.google.com/robots.txt
      initial_alive: false
      timeout: 30
  - addr: 172.16.100.254:1086
    check_config:
      check_url: https://www.google.com/robots.txt
      initial_alive: true
      timeout: 3
```

`server.socks5.addr` is required by the CLI. An empty or `null` configuration
file, invalid backend addresses (empty or whitespace-only), and duplicate
backend addresses are rejected during initialization.

Backend `username` and `password` fields are preserved for authenticated health
checks and the legacy transparent proxy API. The normal SOCKS5 listener forwards
the client's SOCKS5 negotiation unchanged; clients must supply any credentials
required by the selected upstream.

`initial_alive` takes effect immediately for configured and dynamically added
backends. With a `check_url`, periodic HEAD requests mark HTTP 2xx/3xx responses
healthy and do not follow redirects. `timeout` is in seconds and defaults to 10;
it bounds the health request, including SOCKS5 negotiation. Without `check_url`,
the backend retains its configured `initial_alive` policy.

### Environment Variables

- `SELECT_TIME_INTERVAL` - Backend switching interval for the deprecated `ListenTProxy` API only (default: 300 seconds / 5 minutes). Normal SOCKS5 connections use round-robin selection per connection.
- `CHECK_TIME_INTERVAL` - Health check polling interval in seconds (default: 60 seconds / 1 minute)
- `DEBUG` - Enable debug mode (true/false)

## Deployment

### Docker Compose

It's recommended to use `network_mode: 'host'` to avoid network connectivity issues caused by Docker's iptables rules:

```yaml
version: "3"
services:
  socks5lb:
    image: ghcr.io/mingcheng/socks5lb:latest
    restart: always
    dns:
      - 8.8.8.8
      - 8.8.4.4
    environment:
      TZ: "Asia/Shanghai"
      CHECK_TIME_INTERVAL: 3600
    network_mode: "host"
    privileged: true
    volumes:
      - ./socks5lb.yml:/etc/socks5lb.yml:ro
```

### iptables Configuration

The CLI does not start the deprecated transparent proxy listener, and setting
`server.tproxy.addr` alone does not enable it. Library users must explicitly call
`ListenTProxy` on Linux with the required network capabilities. IPv4 original
destination lookup is supported.

For an explicitly started listener on port 8848, example iptables rules follow
(the `redrock` ipset must be configured separately):

```shell
iptables -t nat -I PREROUTING -p tcp -m set --match-set redrock dst -j REDIRECT --to-ports 8848
iptables -t nat -I OUTPUT -p tcp -m set --match-set redrock dst -j REDIRECT --to-ports 8848
```

## Web Management API

Version 1.1.0 introduced a simple web management interface for dynamic proxy configuration:

The management API has no built-in authentication. Bind it to a trusted
interface or protect it with an authenticated reverse proxy.

### GET `/version`

Returns current version, build time, and uptime information.

### GET `/api/all`

Lists all configured proxy servers in address order. Add `?healthy=true` to show
only healthy backends. Invalid boolean filter values return HTTP 400.

### PUT `/api/add`

Adds new proxy backends. The request body should be a JSON array of backend configurations:

```json
[
  {
    "addr": "192.168.1.254:1086",
    "check_config": {
      "check_url": "https://www.taobao.com/robots.txt"
    }
  },
  {
    "addr": "192.168.1.254:1087",
    "check_config": {
      "initial_alive": true
    }
  }
]
```

Returns the number of backends successfully added. Additions are atomic: if an
address already exists or is duplicated within the request, HTTP 503 is returned
and the pool is unchanged. Existing backends must be deleted before re-adding.
Malformed JSON, missing addresses, and empty or `null` batches return HTTP 400.

Example using curl:

```
curl -X "PUT" "<your-address>/api/add" \
     -H 'Content-Type: text/plain; charset=utf-8' \
     -d $'[
  {
    "addr": "192.168.1.1:1086",
    "check_config": {
      "check_url": "https://www.taobao.com/robots.txt"
    }
  }
]'
```

### DELETE `/api/delete`

Removes a specific proxy backend by address using the `addr` query parameter:

```
curl -X "DELETE" "http://localhost:8080/api/delete?addr=192.168.1.1:1086"
```

## FAQ

### How do I disable health checks for a specific backend?

Leave the `check_url` parameter empty and set `initial_alive` to `true`:

```yaml
backends:
  - addr: 127.0.0.1:10860
    check_config:
      initial_alive: true
```

### Can I use the tproxy_listen configuration on non-Linux systems?

No, transparent proxy support is Linux-only. Leave the tproxy configuration empty on other platforms.

### Are there similar projects?

- https://github.com/ginuerzh/gost
- https://github.com/nadoo/glider

## License

MIT License - see [LICENSE](LICENSE) file for details.
