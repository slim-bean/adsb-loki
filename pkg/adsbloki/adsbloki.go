package adsbloki

import (
	"encoding/json"
	"github.com/grafana/loki/clients/pkg/promtail/api"
	"github.com/grafana/loki/pkg/logproto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/slim-bean/adsb-loki/pkg/piaware"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/common/model"
	"github.com/slim-bean/adsb-loki/pkg/aircraft"

	"github.com/grafana/loki/clients/pkg/promtail/client"
	"github.com/grafana/loki/pkg/util/flagext"

	"github.com/slim-bean/adsb-loki/pkg/cfg"
)

type aDSBLoki struct {
	config   *cfg.Config
	logger   log.Logger
	client   client.Client
	pi       *piaware.Piaware
	shutdown chan struct{}
	done     chan struct{}
}

func NewADSBLoki(logger log.Logger, cfg *cfg.Config, am *aircraft.Manager) (*aDSBLoki, error) {
	c, err := client.NewMulti(prometheus.DefaultRegisterer, logger, flagext.LabelSet{}, cfg.ClientConfigs...)
	if err != nil {
		level.Error(logger).Log("msg", "failed to create new Loki client(s)", "err", err)
		return nil, err
	}

	pa := piaware.New(am, cfg.ADSBURL)

	adsb := &aDSBLoki{
		config:   cfg,
		logger:   log.With(logger, "component", "adsbloki"),
		client:   c,
		pi:       pa,
		shutdown: make(chan struct{}),
		done:     make(chan struct{}),
	}

	go adsb.run()
	level.Info(logger).Log("msg", "initialized")
	return adsb, nil
}

func (a *aDSBLoki) run() {
	t := time.NewTicker(time.Second)
	defer func() {
		t.Stop()
		level.Info(a.logger).Log("msg", "run loop shut down")
		close(a.done)
	}()
	level.Info(a.logger).Log("msg", "run loop started")
	for {
		select {
		case <-a.shutdown:
			level.Info(a.logger).Log("msg", "run loop shutting down")
			return
		case <-t.C:
			rpt, err := a.pi.GetReport()
			if err != nil {
				level.Error(a.logger).Log("msg", "error getting report", "err", err)
				continue
			}
			for _, ac := range rpt.Aircraft {
				bts, err := json.Marshal(ac)
				if err != nil {
					level.Error(a.logger).Log("msg", "error getting aircraft info", "err", err)
					continue
				}
				lbls := model.LabelSet{
					model.LabelName("job"): model.LabelValue("adsb"),
					model.LabelName("hex"): model.LabelValue(ac.Hex),
				}
				if ac.Registration != nil {
					lbls[model.LabelName("registration")] = model.LabelValue(*ac.Registration)
				}
				e := api.Entry{
					Labels: lbls,
					Entry: logproto.Entry{
						Timestamp: time.Unix(int64(rpt.Now), 0),
						Line:      string(bts),
					},
				}
				a.client.Chan() <- e
			}
		}
	}
}

func (a *aDSBLoki) Stop() {
	level.Info(a.logger).Log("msg", "shutdown called")
	close(a.shutdown)
	<-a.done
	level.Info(a.logger).Log("msg", "closing clients")
	a.client.Stop()
	level.Info(a.logger).Log("msg", "clients close, shutdown complete")
}
