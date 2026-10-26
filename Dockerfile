FROM golang:1.26.8 AS builder
LABEL maintainer="mingcheng<mingcheng@outook.com>"

WORKDIR /src
ARG GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=1.4.0
ARG BUILD_COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 go build -trimpath -tags timetzdata \
    -ldflags="-s -w -X github.com/mingcheng/socks5lb.Version=${VERSION} -X github.com/mingcheng/socks5lb.BuildCommit=${BUILD_COMMIT} -X github.com/mingcheng/socks5lb.BuildDate=${BUILD_DATE}" \
    -o /out/socks5lb ./cmd/socks5lb

FROM scratch
ENV TZ=Asia/Shanghai
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/socks5lb /bin/socks5lb
COPY --from=builder /src/LICENSE /LICENSE
USER 65532:65532
ENTRYPOINT ["/bin/socks5lb"]
