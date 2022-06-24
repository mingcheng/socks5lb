package main

import (
	"flag"
	"fmt"
	"github.com/mingcheng/socks5lb"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"strings"
)

var (
	pool    *socks5lb.Pool
	servers string
	listen  string
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.TraceLevel)

	flag.StringVar(&servers, "s", "", "socks5 proxy list")
	flag.StringVar(&listen, "l", "127.0.0.1:1080", "local listen tcp port")
}

func main() {
	flag.Parse()

	log.Tracef("new initial backend pools")
	pool = socks5lb.NewPool()

	for _, s := range strings.Split(servers, ",") {
		addr, err := net.ResolveTCPAddr("tcp", s)
		if err != nil {
			log.Error(err)
			continue
		}

		log.Debugf("add backend server, address is %s:%d", addr.IP, addr.Port)
		pool.Add(&socks5lb.Backend{
			Addr: fmt.Sprintf("%s:%d", addr.IP, addr.Port),
		})
	}

	server := socks5lb.Server{
		Pool: pool,
	}
	defer server.Stop()

	if err := server.Start(listen); err != nil {
		log.Panic(err)
	}
}
