package adsbloki

import (
	"encoding/json"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/grafana/loki/pkg/promtail/client"
	"github.com/grafana/loki/pkg/util/flagext"
	"github.com/prometheus/common/model"
	"github.com/wz2b/dump1090-go-client/pkg/dump1090"

	"adsb-loki/pkg/cfg"
)

type aDSBLoki struct {
	config   *cfg.Config
	logger   log.Logger
	client   client.Client
	shutdown chan struct{}
	done     chan struct{}
}

func NewADSBLoki(logger log.Logger, cfg *cfg.Config) (*aDSBLoki, error) {
	c, err := client.NewMulti(logger, flagext.LabelSet{}, cfg.ClientConfigs...)
	if err != nil {
		level.Error(logger).Log("msg", "failed to create new Loki client(s)", "err", err)
		return nil, err
	}

	adsb := &aDSBLoki{
		config: cfg,
		logger: logger,
		client: c,
	}

	go adsb.run()
	level.Info(logger).Log("msg", "adsbloki initialized")
	return adsb, nil
}

func (a *aDSBLoki) run() {
	t := time.NewTicker(time.Second)
	defer func() {
		t.Stop()
		level.Info(a.logger).Log("msg", "run loop shut down")
		close(a.done)
	}()
	level.Info(a.logger).Log("msg", "adsbloki run loop started")
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
				bts, err := json.Marshal(aircraft)
				if err != nil {
					level.Error(a.logger).Log("msg", "error getting aircraft info", "err", err)
				}
				lbls := model.LabelSet{
					model.LabelName("job"): model.LabelValue("adsb"),
					model.LabelName("hex"): model.LabelValue(aircraft.Hex),
				}
				err = a.client.Handle(lbls, time.Unix(int64(rpt.Now), 0), string(bts))
				if err != nil {

				}
			}
		}
	}
}

func (a *aDSBLoki) Stop() {
	level.Info(a.logger).Log("msg", "adsbloki shutdown called")
	close(a.shutdown)
	<-a.done
	level.Info(a.logger).Log("msg", "adsbloki closing clients")
	a.client.Stop()
	level.Info(a.logger).Log("msg", "adsbloki clients close, shutdown complete")
}
