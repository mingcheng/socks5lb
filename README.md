# socks5lb - Simple SOCKS5 Proxy Load Balancer

![socks5lb](./asserts/socks5lb.png)

Sometimes SOCKS5 proxies become unreachable due to network fluctuations or routing changes, requiring manual node switching which can be tedious and time-consuming.

This tool solves that problem by acting as a front-end load balancer for SOCKS5 proxies, automatically routing traffic through verified, healthy proxy nodes.

For Linux systems, it also provides transparent proxy capabilities with SOCKS5 protocol conversion, making it easy to integrate with ipset and iptables.

## Key Features

- **Load Balancing**: Round-robin distribution across SOCKS5 proxies with automatic health checks
- **Transparent Proxy**: Linux [TPROXY](https://www.kernel.org/doc/Documentation/networking/tproxy.txt) support with SOCKS5 protocol conversion
- **Cross-Platform**: Written in Go for easy deployment across platforms (including routers)

## Changelog

- `2025-10-07` Refactored codebase to update copyright information, improve health check mechanisms, and enhance concurrency handling with atomic operations and mutexes. Bump Go version to 1.25 and optimize HTTP server configurations.
- `2022-07-16` Fixed connection performance issues, added HTTP management interface
- `2022-07-06` Completed Linux transparent gateway functionality
- `2022-06-20` Initial release with core features

## Building

The recommended way to build is using docker-compose:

```bash
docker-compose build
```

## Configuration

Here's a basic configuration example with three SOCKS5 proxies exposed on local port 1080, and transparent proxy on port 8848 for Linux systems:

```yaml
server:
  http:
    addr: ":8080"
  socks5:
    addr: ":1080"
  tproxy:
    addr: ":8848"
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

### Environment Variables

- `SELECT_TIME_INTERVAL` - Automatic proxy switching interval in seconds (default: 300 seconds / 5 minutes)
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

Example iptables rules to redirect traffic through port 8848 (note: the `redrock` ipset must be configured separately):

```shell
iptables -t nat -I PREROUTING -p tcp -m set --match-set redrock dst -j REDIRECT --to-ports 8848
iptables -t nat -I OUTPUT -p tcp -m set --match-set redrock dst -j REDIRECT --to-ports 8848
```

## Web Management API

Version 1.1.0 introduced a simple web management interface for dynamic proxy configuration:

### GET `/version`

Returns current version, build time, and uptime information.

### GET `/api/all`

Lists all configured proxy servers. Add `?healthy=true` parameter to show only healthy backends.

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

Returns the number of backends successfully added. Note: Existing backends must be deleted before re-adding.

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
