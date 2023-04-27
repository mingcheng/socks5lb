/**
 * File: main.go
 * Author: Ming Cheng<mingcheng@outlook.com>
 *
 * Created Date: Wednesday, June 22nd 2022, 12:39:47 pm
 * Last Modified: Thursday, February 16th 2023, 3:47:25 pm
 *
 * http://www.opensource.org/licenses/MIT
 */

package main

import (
	"flag"
	"fmt"
	"syscall"

	"github.com/judwhite/go-svc"
	"github.com/mingcheng/socks5lb"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"os"
)

var (
	config  *socks5lb.Configure
	err     error
	cfgPath string
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.ErrorLevel)

	if socks5lb.DebugMode {
		log.SetLevel(log.TraceLevel)
		log.Debug("debug mode is on, its will makes more noise on terminal")
	}

	flag.StringVar(&cfgPath, "c", fmt.Sprintf("/etc/%s.yml", socks5lb.AppName), "the socks5lb configure file path")
}

// NewConfig returns a new Config instance
func NewConfig(path string) (config *socks5lb.Configure, err error) {
	var data []byte

	if data, err = os.ReadFile(path); err != nil {
		return
	}

	if err = yaml.Unmarshal(data, &config); err != nil {
		return
	}

	return
}

func main() {
	usageInfo := fmt.Sprintf("%s v%s(%s), build on %s", socks5lb.AppName, socks5lb.Version, socks5lb.BuildCommit, socks5lb.BuildDate)
	log.Info(usageInfo)

	// overwrite the default usage info
	flag.Usage = func() {
		fmt.Println(usageInfo)
		flag.PrintDefaults()
	}

	// parse the command line flags
	flag.Parse()

	// read the config if err != nil
	if config, err = NewConfig(cfgPath); err != nil {
		log.Fatal(err)
	}

	// call svc run to start the service
	if err := svc.Run(&program{
		Config: config,
	}, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill); err != nil {
		log.Fatal(err)
	}
}
