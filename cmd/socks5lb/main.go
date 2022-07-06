package main

import (
	"flag"
	"github.com/judwhite/go-svc"
	"github.com/mingcheng/socks5lb"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"syscall"

	"os"
)

const AppName = "socks5lb"

var config *socks5lb.Configure
var err error
var configFilePath string

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.TraceLevel)

	flag.StringVar(&configFilePath, "c", "/etc/"+AppName+".yml", "configure file path")
}

func NewConfig(path string) (config *socks5lb.Configure, err error) {
	var (
		data []byte
	)

	if data, err = ioutil.ReadFile(path); err != nil {
		return
	}

	if err = yaml.Unmarshal(data, &config); err != nil {
		return
	}

	return
}

func main() {
	flag.Parse()

	if config, err = NewConfig(configFilePath); err != nil {
		log.Fatal(err)
	}

	// Call svc.Run to start your Program/service.
	if err := svc.Run(&program{
		Config: config,
	}, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill); err != nil {
		log.Fatal(err)
	}
}
