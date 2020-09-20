package cfg

import (
	"flag"

	"github.com/grafana/loki/pkg/promtail/client"

	"adsb-loki/pkg/registration"
)

type Config struct {
	ClientConfigs    []client.Config               `yaml:"clients,omitempty"`
	ADSBURL          string                        `yaml:"adsb_url"`
	RegManagerConfig registration.RegManagerConfig `yaml:"reg_manager,omitempty"`
}

// RegisterFlags with prefix registers flags where every name is prefixed by
// prefix. If prefix is a non-empty string, prefix should end with a period.
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	for i := range c.ClientConfigs {
		c.ClientConfigs[i].RegisterFlags(f)
	}
	c.RegManagerConfig.RegisterFlags(f)
}
