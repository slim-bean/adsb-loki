package adsbloki

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/grafana/loki/pkg/promtail/client"
	"github.com/grafana/loki/pkg/util/flagext"
	"github.com/prometheus/common/model"
	"github.com/wz2b/dump1090-go-client/pkg/dump1090"

	"adsb-loki/pkg/cfg"
	"adsb-loki/pkg/registration"
)

type aDSBLoki struct {
	config   *cfg.Config
	logger   log.Logger
	client   client.Client
	lookup   registration.DetailLookup
	shutdown chan struct{}
	done     chan struct{}
}

func NewADSBLoki(logger log.Logger, cfg *cfg.Config, lookup registration.DetailLookup) (*aDSBLoki, error) {
	c, err := client.NewMulti(logger, flagext.LabelSet{}, cfg.ClientConfigs...)
	if err != nil {
		level.Error(logger).Log("msg", "failed to create new Loki client(s)", "err", err)
		return nil, err
	}

	adsb := &aDSBLoki{
		config: cfg,
		logger: log.With(logger, "component", "adsbloki"),
		client: c,
		lookup: lookup,
		shutdown: make(chan struct{}),
		done: make(chan struct{}),
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
			rpt, err := dump1090.GetReport(a.config.ADSBURL)
			if err != nil {
				level.Error(a.logger).Log("msg", "error getting aircraft info", "err", err)
			}
			for _, aircraft := range rpt.Aircraft {
				details := a.lookup.Lookup(strings.ToLower(aircraft.Hex))
				if details != nil {
					aircraft.Registration = &details.NNumber
					aircraft.AircraftType = &details.TypeAircraft
					aircraft.Description = &details.Name
				}

				bts, err := json.Marshal(aircraft)
				if err != nil {
					level.Error(a.logger).Log("msg", "error getting aircraft info", "err", err)
				}
				lbls := model.LabelSet{
					model.LabelName("job"): model.LabelValue("adsb"),
					model.LabelName("hex"): model.LabelValue(aircraft.Hex),
				}
				if aircraft.Registration != nil {
					lbls[model.LabelName("registration")] = model.LabelValue(*aircraft.Registration)
				}
				err = a.client.Handle(lbls, time.Unix(int64(rpt.Now), 0), string(bts))
				if err != nil {
					level.Error(a.logger).Log("msg", "failed to send to Loki", "err", err)
				}
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
