package adsbloki

import (
	"encoding/json"
	"github.com/grafana/loki/clients/pkg/promtail/api"
	"github.com/grafana/loki/pkg/logproto"
	"github.com/prometheus/client_golang/prometheus"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/common/model"
	"github.com/slim-bean/adsb-loki/pkg/aircraft"

	"github.com/grafana/loki/clients/pkg/promtail/client"
	"github.com/grafana/loki/pkg/util/flagext"

	"github.com/slim-bean/adsb-loki/pkg/cfg"
	ac_model "github.com/slim-bean/adsb-loki/pkg/model"
)

type aDSBLoki struct {
	config   *cfg.Config
	logger   log.Logger
	client   client.Client
	am       *aircraft.Manager
	shutdown chan struct{}
	done     chan struct{}
}

func NewADSBLoki(logger log.Logger, cfg *cfg.Config, am *aircraft.Manager) (*aDSBLoki, error) {
	c, err := client.NewMulti(prometheus.DefaultRegisterer, logger, flagext.LabelSet{}, cfg.ClientConfigs...)
	if err != nil {
		level.Error(logger).Log("msg", "failed to create new Loki client(s)", "err", err)
		return nil, err
	}

	adsb := &aDSBLoki{
		config:   cfg,
		logger:   log.With(logger, "component", "adsbloki"),
		client:   c,
		am:       am,
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
			rpt, err := GetReport(a.config.ADSBURL)
			if err != nil {
				level.Error(a.logger).Log("msg", "error getting aircraft info", "err", err)
				continue
			}
			for _, aircraft := range rpt.Aircraft {
				details := a.am.Lookup(strings.ToLower(aircraft.Hex))
				if details != nil {
					aircraft.Details = *details
				}
				bts, err := json.Marshal(aircraft)
				if err != nil {
					level.Error(a.logger).Log("msg", "error getting aircraft info", "err", err)
					continue
				}
				lbls := model.LabelSet{
					model.LabelName("job"): model.LabelValue("adsb"),
					model.LabelName("hex"): model.LabelValue(aircraft.Hex),
				}
				if aircraft.Registration != nil {
					lbls[model.LabelName("registration")] = model.LabelValue(*aircraft.Registration)
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

func GetReport(url string) (*ac_model.Report, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	report := &ac_model.Report{}
	err = json.Unmarshal(body, report)
	if err != nil {
		return nil, err
	}

	/*
	 * Clean up the flight ID by removing leading and trailing spaces
	 */
	for i, a := range report.Aircraft {
		if a.Flight != nil {
			trimmed := strings.TrimSpace(*a.Flight)
			report.Aircraft[i].Flight = &trimmed
		}

		//fmt.Println(reflect.TypeOf(a.alt_baro))
	}

	return report, nil
}
