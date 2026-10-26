/**
 * File: main.go
 * Author: Ming Cheng<mingcheng@outlook.com>
 *
 * Created Date: Wednesday, June 22nd 2022, 12:39:47 pm
 * Last Modified: Friday, July 15th 2022, 5:53:09 pm
 *
 * http://www.opensource.org/licenses/MIT
 */

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/judwhite/go-svc"
	"github.com/mingcheng/socks5lb"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var cfgPath string

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.ErrorLevel)

	if socks5lb.DebugMode {
		log.SetLevel(log.TraceLevel)
		log.Debug("debug mode is enabled, it will print more logs.")
	}

	flag.StringVar(&cfgPath, "c", "/etc/"+socks5lb.AppName+".yml", "configure file cfgPath")
}

// NewConfig returns a new Config instance
func NewConfig(path string) (*socks5lb.Configure, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read configuration %s: %w", path, err)
	}

	var config *socks5lb.Configure
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("decode configuration %s: %w", path, err)
	}
	if config == nil {
		return nil, fmt.Errorf("configuration %s is empty", path)
	}
	if strings.TrimSpace(config.ServerConfig.Sock5.Addr) == "" {
		return nil, fmt.Errorf("configuration %s: server.socks5.addr is required", path)
	}
	if err := config.ServerConfig.Validate(); err != nil {
		return nil, fmt.Errorf("configuration %s: %w", path, err)
	}
	return config, nil
}

func main() {
	log.Infof("%s v%s(%s), build on %s", socks5lb.AppName, socks5lb.Version, socks5lb.BuildCommit, socks5lb.BuildDate)
	flag.Parse()

	config, err := NewConfig(cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	// Call svc.Run to start your Program/service.
	if err := svc.Run(&program{
		Config: config,
	}); err != nil {
		log.Fatal(err)
	}
}
