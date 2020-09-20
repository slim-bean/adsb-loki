package registration

import (
	"archive/zip"
	"flag"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/dimchansky/utfbom"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gocarina/gocsv"
)

const (
	regfile     = "ReleasableAircraft.zip"
	tempRegfile = "ReleasableAircraft.zip.tmp"
)

//N-NUMBER,SERIAL NUMBER,MFR MDL CODE,ENG MFR MDL,YEAR MFR,TYPE REGISTRANT,NAME,STREET,STREET2,CITY,STATE,ZIP CODE,REGION,COUNTY,COUNTRY,LAST ACTION DATE,CERT ISSUE DATE,CERTIFICATION,TYPE AIRCRAFT,TYPE ENGINE,STATUS CODE,MODE S CODE,FRACT OWNER,AIR WORTH DATE,OTHER NAMES(1),OTHER NAMES(2),OTHER NAMES(3),OTHER NAMES(4),OTHER NAMES(5),EXPIRATION DATE,UNIQUE ID,KIT MFR, KIT MODEL,MODE S CODE HEX,

type Detail struct {
	NNumber         string `csv:"N-NUMBER"`
	SerialNumber    string `csv:"SERIAL NUMBER"`
	MfrModelCode    string `csv:"MFR MDL CODE"`
	EngMfrModel     string `csv:"ENG MFR MDL"`
	YearMfr         string `csv:"YEAR MFR"`
	TypeRegistratnt string `csv:"TYPE REGISTRANT"`
	Name            string `csv:"NAME"`
	Street          string `csv:"STREET"`
	Street2         string `csv:"STREET2"`
	City            string `csv:"CITY"`
	State           string `csv:"STATE"`
	ZipCode         string `csv:"ZIP CODE"`
	Region          string `csv:"REGION"`
	County          string `csv:"COUNTY"`
	Country         string `csv:"COUNTRY"`
	LastActionDate  string `csv:"LAST ACTION DATE"`
	CertIssueDate   string `csv:"CERT ISSUE DATE"`
	Certification   string `csv:"CERTIFICATION"`
	TypeAircraft    string `csv:"TYPE AIRCRAFT"`
	TypeEngine      string `csv:"TYPE ENGINE"`
	StatusCode      string `csv:"STATUS CODE"`
	ModeSCode       string `csv:"MODE S CODE"`
	FractOwner      string `csv:"FRACT OWNER"`
	AirWorthDate    string `csv:"AIR WORTH DATE"`
	OtherNames1     string `csv:"OTHER NAMES(1)"`
	OtherNames2     string `csv:"OTHER NAMES(2)"`
	OtherNames3     string `csv:"OTHER NAMES(3)"`
	OtherNames4     string `csv:"OTHER NAMES(4)"`
	OtherNames5     string `csv:"OTHER NAMES(5)"`
	ExpirationDate  string `csv:"EXPIRATION DATE"`
	UniqueID        string `csv:"UNIQUE ID"`
	KitMfr          string `csv:"KIT MFR"`
	KitModel        string `csv:"KIT MODEL"`
	ModeSCodeHex    string `csv:"MODE S CODE HEX"`
}

type DetailLookup interface {
	Lookup(hex string) *Detail
}

type RegManagerConfig struct {
	Directory string `yaml:"directory"`
	URL       string `yaml:"url"`
}

func (c *RegManagerConfig) RegisterFlags(f *flag.FlagSet) {
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	f.StringVar(&c.Directory, "reg-manager.directory", path, "Where to save the downloaded registration zip file, defaults to the current working directory")
	f.StringVar(&c.URL, "req-manager.url", "http://registry.faa.gov/database/ReleasableAircraft.zip", "Where to get aircraft information")
}

type manager struct {
	logger     log.Logger
	config     RegManagerConfig
	deteMap    map[string]*Detail
	deteMapMtx sync.Mutex
	shutdown   chan struct{}
	done       chan struct{}
}

func NewManager(logger log.Logger, config RegManagerConfig) (*manager, error) {
	m := &manager{
		logger: log.With(logger, "component", "manager"),
		config: config,
	}

	gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
		return gocsv.LazyCSVReader(in) // Allows use of quotes in CSV
	})

	m.checkAndUpdateRegistrationFile()
	m.loadRegistrationInfo()
	go m.run()
	level.Info(logger).Log("msg", "mananger initialized")
	return m, nil
}

func (m *manager) run() {
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

func (m *manager) Lookup(hex string) *Detail {
	m.deteMapMtx.Lock()
	defer m.deteMapMtx.Unlock()
	return m.deteMap[hex]
}

func (m *manager) Stop() {
	level.Info(m.logger).Log("msg", "stop called")
	close(m.shutdown)
	<-m.done
	level.Info(m.logger).Log("msg", "shutdown complete")
}

func (m *manager) checkAndUpdateRegistrationFile() bool {
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

func (m *manager) loadRegistrationInfo() {
	r, err := zip.OpenReader(path.Join(m.config.Directory, regfile))
	if err != nil {
		level.Error(m.logger).Log("msg", "failed to open downloaded zip file", "err", err)
		return
	}
	defer r.Close()

	// Look through files until we find master file
	for _, zipFile := range r.File {
		if zipFile.Name != "MASTER.txt" {
			continue
		}
		level.Info(m.logger).Log("msg", "found MASTER.txt in downloaded zip file, updating aircraft details in memory")
		f, err := zipFile.Open()
		if err != nil {
			level.Error(m.logger).Log("msg", "failed to open master aircraft file from downloaded zip file", "err", err)
			return
		}
		defer f.Close()

		bts, err := ioutil.ReadAll(utfbom.SkipOnly(f))
		if err != nil {
			level.Error(m.logger).Log("msg", "failed to read bytes from master aircraft file", "err", err)
			return
		}

		details := []*Detail{}
		//err = csvutil.Unmarshal(bts, &details)
		err = gocsv.UnmarshalBytes(bts, &details)
		if err != nil {
			level.Error(m.logger).Log("msg", "failed to parse registration file into csv", "err", err)
			return
		}

		nMap := map[string]*Detail{}
		for i := range details {
			details[i].NNumber = "N" + details[i].NNumber
			details[i].Name = strings.TrimSpace(details[i].Name)
			nMap[strings.TrimSpace(strings.ToLower(details[i].ModeSCodeHex))] = details[i]
		}
		m.deteMapMtx.Lock()
		m.deteMap = nMap
		m.deteMapMtx.Unlock()
		level.Info(m.logger).Log("msg", "finished updating aircraft registration details", "mapLength", len(m.deteMap))
	}
}
