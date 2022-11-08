FROM golang:1.19 AS builder
LABEL maintainer="mingcheng<mingcheng@outook.com>"

ENV PACKAGE github.com/mingcheng/socks5lb
ENV GOPROXY https://goproxy.cn,direct
ENV BUILD_DIR ${GOPATH}/src/${PACKAGE}

# Build
COPY . ${BUILD_DIR}
WORKDIR ${BUILD_DIR}
RUN make build && mv ./socks5lb /usr/bin/socks5lb

# Stage2
FROM debian:bullseye

ENV TZ "Asia/Shanghai"
RUN sed -i 's/deb.debian.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apt/sources.list \
	&& sed -i 's/security.debian.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apt/sources.list \
	&& echo "Asia/Shanghai" > /etc/timezone \
	&& apt -y update \
	&& apt -y upgrade \
	&& apt -y install ca-certificates openssl tzdata curl dumb-init \
	&& apt -y autoremove

COPY --from=builder /usr/bin/socks5lb /bin/socks5lb

ENTRYPOINT ["dumb-init", "/bin/socks5lb"]
