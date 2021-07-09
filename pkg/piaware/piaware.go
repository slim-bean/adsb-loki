package piaware

import (
	"encoding/json"
	"github.com/slim-bean/adsb-loki/pkg/aircraft"
	"github.com/slim-bean/adsb-loki/pkg/model"
	"io/ioutil"
	"net/http"
	"strings"
)

type Piaware struct {
	url string
	am  *aircraft.Manager
}

func New(am *aircraft.Manager, url string) *Piaware {
	return &Piaware{
		url: url,
		am:  am,
	}
}

func (p *Piaware) GetReport() (*model.Report, error) {
	rpt, err := p.getReport()
	if err != nil {
		return nil, err
	}

	for i, ac := range rpt.Aircraft {
		details := p.am.Lookup(strings.ToLower(ac.Hex))
		if details != nil {
			rpt.Aircraft[i].Details = *details
		}
	}

	return rpt, nil
}

func (p *Piaware) getReport() (*model.Report, error) {
	resp, err := http.Get(p.url)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	report := &model.Report{}
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
