/**
 * File: main.go
 * Author: Ming Cheng<mingcheng@outlook.com>
 *
 * Created Date: Wednesday, June 22nd 2022, 12:39:47 pm
 * Last Modified: Thursday, July 7th 2022, 6:29:42 pm
 *
 * http://www.opensource.org/licenses/MIT
 */

package main

import (
	"flag"
	"io/ioutil"
	"syscall"

	"github.com/judwhite/go-svc"
	"github.com/mingcheng/socks5lb"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"os"
)

const AppName = "socks5lb"

var (
	config  *socks5lb.Configure
	err     error
	cfgPath string
)

var (
	version = "n/a"
	commit  = "n/a"
	date    = "n/a"
)

func init() {
	log.SetOutput(os.Stdout)

	isDebug := socks5lb.GetEnv("DEBUG", "")
	if isDebug != "" {
		log.SetLevel(log.TraceLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	flag.StringVar(&cfgPath, "c", "/etc/"+AppName+".yml", "configure file cfgPath")
}

// NewConfig returns a new Config instance
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
	log.Infof("%s v%s(%s), build on %s", AppName, version, commit, date)

	flag.Parse()

	// read the config if err != nil
	if config, err = NewConfig(cfgPath); err != nil {
		log.Fatal(err)
	}

	// Call svc.Run to start your Program/service.
	if err := svc.Run(&program{
		Config: config,
	}, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill); err != nil {
		log.Fatal(err)
	}
}
