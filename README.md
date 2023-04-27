# socks5lb，简单的 Socks5 代理负载均衡

![socks5lb](./asserts/socks5lb.png)

有时候我们在使用 Socks5 Proxy 无法联通的情况，这有可能是因为网络或者线路的调整和波动，这时候往往需要我们自己手工的切换节点，非常的麻烦。

这个工具就是为了解决上述问题而编写的，它简单的说就是个针对 Socks5 Proxy 的前置负载均衡，能够提供经过检验的稳定可靠的 Socks Proxy 节点。

如果是针对 Linux 系统下同时能够提供透明代理以及针对 Socks5
协议的转换，而且方便搭配 ipset 以及 iptables 使用。

目前实现的部分特性有：

- 能够提供 Socks5 Proxy 的负载均衡（轮询机制）同时提供健康检查；
- 针对 Linux 提供[透明代理](https://www.kernel.org/doc/Documentation/networking/tproxy.txt)以及 Socks5 的协议转换；
- 使用 Golang 编写，跨平台部署（例如部署到各种路由器上）和配置方便。

## 更新记录

- `20220716` 修复部分链接的性能问题，增加 HTTP 管理接口
- `20220706` 完成针对 Linux 的透明网关功能
- `20220620` 完成基本功能

## 编译

建议使用 docker-compose 编译生成镜像文件，直接执行 docker-compose build 即可。

## 配置

首先是针对 socks5lb 的基本配置，例如以下的配置配置了三个 Socks5 Proxy 同时暴露到本地的 1080 端口，针对 Linux 的透明代理暴露在 8848 端口。

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

#### 环境变量

- `SELECT_TIME_INTERVAL` 自动切换代理的时间，单位为秒（默认 300 秒，五分钟）
- `CHECK_TIME_INTERVAL` 健康检查的轮询时间，单位为秒（默认一分钟、60 秒）
- `DEBUG` 是否打开 debug 模式

### 部署

首先，以下是 docker-compose 相关的配置，建议使用 `network_mode: 'host'` 方式，防止过多的网络层转换造成网络联通错误：

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

然后配置（供参考）iptable 参数，将所有的流量都通过 8848 代理端口转发（注意 redrock 链没有定义，请自行配置）。

```shell
iptables -t nat -I PREROUTING -p tcp -m set --match-set redrock dst -j REDIRECT --to-ports 8848
iptables -t nat -I OUTPUT -p tcp -m set --match-set redrock dst -j REDIRECT --to-ports 8848
```

### Web 管理

自 1.1.0 版本实现了个简单的 Web 管理接口，用于动态的添加和删除代理服务器的配置，简单的说明如下：

#### GET `/version`

目前运行的版本、编译时间以及运行时间

#### GET `/api/all`

显示目前配置的代理服务器列表，如果加 `healthy=true` 参数，则只显示目前健康的代理节点

#### PUT `/api/add`

增加代理，这里说明下 Put 的 Body 为 JSON 数组，同时配置和代理的配置对应，例如

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

然后返回的是已经加入的代理节点数量（整型数）。如果已经有配置的代理节点，则需要先删除以后再加入。

示例 CURL 如下：

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

#### DELETE `/api/delete`

删除指定的代理地址，参数 `addr` 指定参数名称。

```
curl -X "DELETE" "http://localhost:8080/api/delete?addr=192.168.1.1:1086"
```

## 常见问题

### 如果我不想针对某个节点健康检查呢（强制使用）？

那么可以配置节点 `check_url` 参数为空，然后默认 `initial_alive` 为 `true` 即可，例如：

```yaml
backends:
  - addr: 127.0.0.1:10860
    check_config:
      initial_alive: true
```

### 在其他非 Linux 系统下可以使用 tproxy_listen 这个配置吗？

不好意思，透明代理只针对 Linux 平台，所以如果是非 Linux 平台，请留空对应的配置。

### 有没有类似功能的项目？

- https://github.com/ginuerzh/gost
- https://github.com/nadoo/glider
