package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/cortexproject/cortex/pkg/util/flagext"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	lokiconfig "github.com/grafana/loki/pkg/cfg"
	"github.com/prometheus/common/version"

	"adsb-loki/pkg/adsbloki"
	"adsb-loki/pkg/cfg"
	"adsb-loki/pkg/registration"
)

type Config struct {
	cfg.Config   `yaml:",inline"`
	printVersion bool
	configFile   string
}

func (c *Config) RegisterFlags(f *flag.FlagSet) {
	f.BoolVar(&c.printVersion, "version", false, "Print this builds version information")
	f.StringVar(&c.configFile, "config.file", "", "yaml file to load")
	c.Config.RegisterFlags(f)
}

// Clone takes advantage of pass-by-value semantics to return a distinct *Config.
// This is primarily used to parse a different flag set without mutating the original *Config.
func (c *Config) Clone() flagext.Registerer {
	return func(c Config) *Config {
		return &c
	}(*c)
}

func main() {

	var config Config

	if err := lokiconfig.Parse(&config); err != nil {
		fmt.Fprintf(os.Stderr, "failed parsing config: %v\n", err)
		os.Exit(1)
	}
	if config.printVersion {
		fmt.Println(version.Print("adsb-loki"))
		os.Exit(0)
	}

	var logger log.Logger
	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger = log.With(logger, "ts", log.DefaultTimestamp, "caller", log.DefaultCaller)

	shutdown := make(chan struct{})
	go sig(logger, shutdown)

	m, err := registration.NewManager(logger, config.RegManagerConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init the registration manager: %v\n", err)
		os.Exit(1)
	}

	al, err := adsbloki.NewADSBLoki(logger, &config.Config, m)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init the application: %v\n", err)
		os.Exit(1)
	}

	<-shutdown
	al.Stop()
	level.Info(logger).Log("msg", "shutdown complete")
	os.Exit(0)
}

func sig(logger log.Logger, shutdown chan struct{}) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	buf := make([]byte, 1<<20)
	for {
		select {
		case sig := <-sigs:
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				level.Info(logger).Log("msg", "=== received SIGINT/SIGTERM ===")
				close(shutdown)
				return
			case syscall.SIGQUIT:
				stacklen := runtime.Stack(buf, true)
				level.Info(logger).Log("msg", fmt.Sprintf("=== received SIGQUIT ===\n*** goroutine dump...\n%s\n*** end", buf[:stacklen]))
			}
		}
	}
}
