package aircraft

import (
	"bufio"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gocarina/gocsv"
	"github.com/slim-bean/adsb-loki/pkg/model"
	bolt "go.etcd.io/bbolt"
)

const (
	regfile     = "aircraft.csv.gz"
	tempRegfile = "aircraft.csv.gz.tmp"
)

var (
	trueVar = true
)

type Config struct {
	Directory  string `yaml:"directory"`
	BoltDbFile string `yaml:"db_file"`
	URL        string `yaml:"url"`
}

func (c *Config) RegisterFlags(f *flag.FlagSet) {
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	f.StringVar(&c.Directory, "aircraft-manager.directory", path, "Where to save the downloaded aircraft info, defaults to the current working directory")
	f.StringVar(&c.BoltDbFile, "aircraft-manager.db-file", filepath.Join(path, "aircraft.db"), "Where to save the aircraft db, defaults to the current working directory ./aircraft.db")
	f.StringVar(&c.URL, "aircraft-manager.url", "https://github.com/wiedehopf/tar1090-db/raw/csv/aircraft.csv.gz", "Where to get aircraft information")
}

type Manager struct {
	logger   log.Logger
	config   Config
	db       *bolt.DB
	shutdown chan struct{}
	done     chan struct{}
}

func NewAircraftManager(logger log.Logger, config Config) (*Manager, error) {

	db, err := bolt.Open(config.BoltDbFile, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("error opening boltdb file: %s", err)
	}
	m := &Manager{
		logger: log.With(logger, "component", "manager"),
		config: config,
		db:     db,
	}

	gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
		r := csv.NewReader(in)
		r.LazyQuotes = true
		r.Comma = ';'
		return r
	})
	level.Info(logger).Log("msg", "mananger initialized")
	return m, nil
}

func (m *Manager) Run() {
	m.checkAndUpdateRegistrationFile()
	m.loadRegistrationInfo()
	go m.run()
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
	var d *model.Details
	err := m.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("aircraft"))
		v := b.Get([]byte(hex))
		if v != nil {
			d = &model.Details{}
			err := json.Unmarshal(v, d)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		level.Error(m.logger).Log("msg", "failed to retrieve aircraft info from boltdb", "err", err)
	}
	return d
}

func (m *Manager) Stop() {
	level.Info(m.logger).Log("msg", "stop called")
	close(m.shutdown)
	m.db.Close()
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

	defer os.Remove(out.Name())

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		level.Error(m.logger).Log("msg", "failed to copy file to temp file", "err", err)
		return false
	}

	out.Close()

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
	jp := NewCsvParser(reader)
	err = m.db.Update(func(tx *bolt.Tx) error {
		_ = tx.DeleteBucket([]byte("aircraft"))
		b, err := tx.CreateBucket([]byte("aircraft"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		for jp.Next() {
			h, d := jp.Details()
			ac, err := json.Marshal(d)
			if err != nil {
				level.Warn(m.logger).Log("msg", "failed to marshal aircraft details into json for storage in boltdb", "err", err)
				continue
			}
			err = b.Put([]byte(h), ac)
			if err != nil {
				return fmt.Errorf("adding key: %s", err)
			}
		}
		return nil
	})
	if err != nil {
		level.Error(m.logger).Log("msg", "errors updating boltdb database with new info", "err", err)
		return
	}
	level.Info(m.logger).Log("msg", "finished updating aircraft registration details")

}

// CsvParser is built to parse the CSV file at github.com/wiedehopf/tar1090-db/raw/csv/aircraft.csv.gz
// It's also built to do this while minimizing allocations and as such does some risky slice->string conversions
// The custome CsvParser was built to work around the non-standard CSV format created by this python
// spamwriter = csv.writer(csvfile,
//                delimiter=';', escapechar='\\',
//                quoting=csv.QUOTE_NONE, quotechar=None,
//                lineterminator='\n')
// Which is semicolon delimited, no header, non quoted and uses backslashes to escape
// There currently aren't any lines in the file that use the escape char so I'm not 100% sure the code here handles that correctly.
type CsvParser struct {
	s             *bufio.Scanner
	details       *model.Details
	hex           string
	hexSlice      []byte
	regSlice      []byte
	typeCodeSlice []byte
	optCodeSlice  []byte
	descSlice     []byte
	manSlice      []byte
	ownSlice      []byte
}

func NewCsvParser(r io.Reader) *CsvParser {
	return &CsvParser{
		s:             bufio.NewScanner(r),
		details:       &model.Details{},
		hexSlice:      make([]byte, 1000),
		regSlice:      make([]byte, 1000),
		typeCodeSlice: make([]byte, 1000),
		optCodeSlice:  make([]byte, 1000),
		descSlice:     make([]byte, 1000),
		manSlice:      make([]byte, 1000),
		ownSlice:      make([]byte, 1000),
	}
}

func (j *CsvParser) Next() bool {
	if !j.s.Scan() {
		return false
	}
	lastPos := 0
	part := 0
	lb := j.s.Bytes()
	//Reset details to be empty
	j.details.Owner = nil
	j.details.PIA = nil
	j.details.Military = nil
	j.details.LADD = nil
	j.details.Interesting = nil
	j.details.Manufactured = nil
	j.details.Description = nil
	j.details.TypeCode = nil
	j.details.Registration = nil
	for p, r := range lb {
		if r == ';' {
			if p > 0 && lb[p-1] == '\\' {
				continue
			}
			var start, end int
			if p-lastPos > 1 {
				if lastPos == 0 {
					start = lastPos
					end = p
				} else {
					start = lastPos + 1
					end = p
				}
				switch part {
				case 0:
					j.hexSlice = j.hexSlice[:0]
					j.hexSlice = append(j.hexSlice, lb[start:end]...)
					j.hex = *(*string)(unsafe.Pointer(&j.hexSlice))
				case 1:
					j.regSlice = j.regSlice[:0]
					j.regSlice = append(j.regSlice, lb[start:end]...)
					j.details.Registration = (*string)(unsafe.Pointer(&j.regSlice))
				case 2:
					j.typeCodeSlice = j.typeCodeSlice[:0]
					j.typeCodeSlice = append(j.typeCodeSlice, lb[start:end]...)
					j.details.TypeCode = (*string)(unsafe.Pointer(&j.typeCodeSlice))
				case 3:
					j.optCodeSlice = j.optCodeSlice[:0]
					j.optCodeSlice = append(j.optCodeSlice, lb[start:end]...)
					if len(j.optCodeSlice) >= 1 {
						if j.optCodeSlice[0] == '1' {
							j.details.Military = &trueVar
						}
					}
					if len(j.optCodeSlice) >= 2 {
						if j.optCodeSlice[1] == '1' {
							j.details.Interesting = &trueVar
						}
					}
					if len(j.optCodeSlice) >= 3 {
						if j.optCodeSlice[2] == '1' {
							j.details.PIA = &trueVar
						}
					}
					if len(j.optCodeSlice) >= 4 {
						if j.optCodeSlice[3] == '1' {
							j.details.LADD = &trueVar
						}
					}
				case 4:
					j.descSlice = j.descSlice[:0]
					j.descSlice = append(j.descSlice, lb[start:end]...)
					j.details.Description = (*string)(unsafe.Pointer(&j.descSlice))
				case 5:
					j.manSlice = j.manSlice[:0]
					j.manSlice = append(j.manSlice, lb[start:end]...)
					j.details.Manufactured = (*string)(unsafe.Pointer(&j.manSlice))
				case 6:
					j.ownSlice = j.ownSlice[:0]
					j.ownSlice = append(j.ownSlice, lb[start:end]...)
					j.details.Owner = (*string)(unsafe.Pointer(&j.ownSlice))
				}
			}
			part++
			lastPos = p

		}
	}
	return true
}

// Details returns a pointer to the current details
// NOTE everything about the returned object is UNSAFE it is intended that this object be serialized to a string immediately before calling Next()
func (j *CsvParser) Details() (string, *model.Details) {
	return strings.TrimSpace(strings.ToLower(j.hex)), j.details
}
