package aircraft

import (
	"bufio"
	"compress/gzip"
	"encoding/csv"
	"flag"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"adsb-loki/pkg/model"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gocarina/gocsv"
)

const (
	regfile     = "aircraft.csv.gz"
	tempRegfile = "aircraft.csv.gz.tmp"
)

var (
	trueVar = true
)

type Config struct {
	Directory string `yaml:"directory"`
	URL       string `yaml:"url"`
}

func (c *Config) RegisterFlags(f *flag.FlagSet) {
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	f.StringVar(&c.Directory, "aircraft-manager.directory", path, "Where to save the downloaded aircraft info, defaults to the current working directory")
	f.StringVar(&c.URL, "aircraft-manager.url", "https://github.com/wiedehopf/tar1090-db/raw/csv/aircraft.csv.gz", "Where to get aircraft information")
}

type Manager struct {
	logger     log.Logger
	config     Config
	deteMap    map[string]*model.Details
	deteMapMtx sync.Mutex
	shutdown   chan struct{}
	done       chan struct{}
}

func NewAircraftManager(logger log.Logger, config Config) (*Manager, error) {
	m := &Manager{
		logger: log.With(logger, "component", "manager"),
		config: config,
	}

	gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
		r := csv.NewReader(in)
		r.LazyQuotes = true
		r.Comma = ';'
		return r
	})

	m.checkAndUpdateRegistrationFile()
	m.loadRegistrationInfo()
	go m.run()
	level.Info(logger).Log("msg", "mananger initialized")
	return m, nil
}

func (m *Manager) run() {
	t := time.NewTicker(time.Minute)
	defer func() {
		t.Stop()
		level.Info(m.logger).Log("msg", "run loop shut down")
		close(m.done)
	}()
	level.Info(m.logger).Log("msg", "run loop started")
	for {
		select {
		case <-m.shutdown:
			level.Info(m.logger).Log("msg", "run loop shutting down")
			return
		case <-t.C:
			if m.checkAndUpdateRegistrationFile() {
				m.loadRegistrationInfo()
			}
		}
	}
}

func (m *Manager) Lookup(hex string) *model.Details {
	m.deteMapMtx.Lock()
	defer m.deteMapMtx.Unlock()
	return m.deteMap[hex]
}

func (m *Manager) Stop() {
	level.Info(m.logger).Log("msg", "stop called")
	close(m.shutdown)
	<-m.done
	level.Info(m.logger).Log("msg", "shutdown complete")
}

func (m *Manager) checkAndUpdateRegistrationFile() bool {
	fi, err := os.Stat(path.Join(m.config.Directory, regfile))
	if err == nil {
		if time.Since(fi.ModTime()) < 24*time.Hour {
			return false
		}
	} else if !os.IsNotExist(err) {
		level.Error(m.logger).Log("msg", "failed to stat registration file, cannot update", "err", err)
		return false
	}

	// File does not exist or it's more than 24 hours old.
	level.Info(m.logger).Log("msg", "downloading new registration file")

	// Get the data
	resp, err := http.Get(m.config.URL)
	if err != nil {
		level.Error(m.logger).Log("msg", "failed to download new registration file", "url", m.config.URL, "err", err)
		return false
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(path.Join(m.config.Directory, tempRegfile))
	if err != nil {
		level.Error(m.logger).Log("msg", "failed to create temp registration file", "err", err)
		return false
	}
	defer out.Close()
	defer os.Remove(out.Name())

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		level.Error(m.logger).Log("msg", "failed to copy file to temp file", "err", err)
		return false
	}

	err = os.Rename(path.Join(m.config.Directory, tempRegfile), path.Join(m.config.Directory, regfile))
	if err != nil {
		level.Error(m.logger).Log("msg", "failed to rename temp registration file to file", "err", err)
		return false
	}

	level.Info(m.logger).Log("msg", "new registration file downloaded and replaced existing file")

	return true
}

func (m *Manager) loadRegistrationInfo() {
	file, err := os.Open(path.Join(m.config.Directory, regfile))
	if err != nil {
		level.Error(m.logger).Log("msg", "failed to open downloaded zip file", "err", err)
		return
	}
	reader, err := gzip.NewReader(file)
	if err != nil {
		level.Error(m.logger).Log("msg", "failed to open gzip reader on file", "err", err)
		return
	}
	defer reader.Close()
	defer file.Close()
	nMap := buildDetails(reader)
	m.deteMapMtx.Lock()
	m.deteMap = nMap
	m.deteMapMtx.Unlock()
	level.Info(m.logger).Log("msg", "finished updating aircraft registration details", "mapLength", len(m.deteMap))

}

func buildDetails(reader io.Reader) map[string]*model.Details {
	lastPos := 0
	part := 0
	hex := ""
	details := model.Details{}
	nMap := map[string]*model.Details{}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		for p, r := range line {
			if r == ';' {
				if p > 0 && line[p-1] == '\\' {
					continue
				}
				var substr string
				if p-lastPos > 1 {
					if lastPos == 0 {
						substr = line[lastPos:p]
					} else {
						substr = line[lastPos+1 : p]
					}
					switch part {
					case 0:
						hex = substr
					case 1:
						details.Registration = &substr
					case 2:
						details.TypeCode = &substr
					case 3:
						if len(substr) >= 1 {
							if substr[0] == '1' {
								details.Military = &trueVar
							}
						}
						if len(substr) >= 2 {
							if substr[1] == '1' {
								details.Interesting = &trueVar
							}
						}
						if len(substr) >= 3 {
							if substr[2] == '1' {
								details.PIA = &trueVar
							}
						}
						if len(substr) >= 4 {
							if substr[3] == '1' {
								details.LADD = &trueVar
							}
						}
					case 4:
						details.Description = &substr
					case 5:
						details.Manufactured = &substr
					case 6:
						details.Owner = &substr
					}
				}
				part++
				lastPos = p
			}
		}
		lastPos = 0
		part = 0
		copyDetails := details
		nMap[strings.TrimSpace(strings.ToLower(hex))] = &copyDetails
		details = model.Details{}
	}
	return nMap
}
