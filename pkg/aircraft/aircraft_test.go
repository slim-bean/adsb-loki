package aircraft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/slim-bean/adsb-loki/pkg/model"
)

var testFile = `38BB7B;F-WIZZ;ZZZZ;00;Issoire Aviation APM-50 Nala;;;
38BC5B;F-AZXS;P51;00;P-51D "Moonbeam McSwine";;;
A08AE3;N134JP;VELO;00;1999 PEARCE JAMES L VELOCITY RG;1999;PLANE FUN INC TR TRUSTEE;
A09200;;;0010;;;;
AE595D;14-5791;C30J;10;Lockheed C-130J-30 Hercules;;;
38BE7B;F-PGMG;D11;00;Jod\;el D.119-D;;;
3EBBB4;3X+XX;EUFI;1100;Eurofighter 2000;;;
1F3342;RF-78658;IL76;0100;Il'yushin Il-76MD-90A;;;
48D980;0110;B738;10;Boeing 737NG 800/W;;;
A0002B;N1BR;C240;0001;Cessna 240;2015;VAN BORTEL AIRCRAFT INC;
`

var expectedDetails = map[string]*model.Details{
	"38bb7b": {
		Registration: stringP("F-WIZZ"),
		TypeCode:     stringP("ZZZZ"),
		Description:  stringP("Issoire Aviation APM-50 Nala"),
	},
	"38bc5b": {
		Registration: stringP("F-AZXS"),
		TypeCode:     stringP("P51"),
		Description:  stringP("P-51D \"Moonbeam McSwine\""),
	},
	"a08ae3": {
		Registration: stringP("N134JP"),
		TypeCode:     stringP("VELO"),
		Description:  stringP("1999 PEARCE JAMES L VELOCITY RG"),
		Manufactured: stringP("1999"),
		Owner:        stringP("PLANE FUN INC TR TRUSTEE"),
	},
	"a09200": {
		PIA: &trueVar,
	},
	"ae595d": {
		Registration: stringP("14-5791"),
		TypeCode:     stringP("C30J"),
		Military:     &trueVar,
		Description:  stringP("Lockheed C-130J-30 Hercules"),
	},
	"38be7b": {
		Registration: stringP("F-PGMG"),
		TypeCode:     stringP("D11"),
		Description:  stringP("Jod\\;el D.119-D"),
	},
	"3ebbb4": {
		Registration: stringP("3X+XX"),
		TypeCode:     stringP("EUFI"),
		Military:     &trueVar,
		Interesting:  &trueVar,
		Description:  stringP("Eurofighter 2000"),
	},
	"1f3342": {
		Registration: stringP("RF-78658"),
		TypeCode:     stringP("IL76"),
		Interesting:  &trueVar,
		Description:  stringP("Il'yushin Il-76MD-90A"),
	},
	"48d980": {
		Registration: stringP("0110"),
		TypeCode:     stringP("B738"),
		Military:     &trueVar,
		Description:  stringP("Boeing 737NG 800/W"),
	},
	"a0002b": {
		Registration: stringP("N1BR"),
		TypeCode:     stringP("C240"),
		LADD:         &trueVar,
		Description:  stringP("Cessna 240"),
		Manufactured: stringP("2015"),
		Owner:        stringP("VAN BORTEL AIRCRAFT INC"),
	},
}

func stringP(v string) *string {
	return &v
}

func Test_JsonParser(t *testing.T) {
	r := strings.NewReader(testFile)
	p := NewCsvParser(r)
	m := map[string][]byte{}
	for p.Next() {
		h, dp := p.Details()
		dpBytes, err := json.Marshal(dp)
		if err != nil {
			panic(err)
		}
		m[h] = dpBytes
	}
	for k, v := range expectedDetails {
		eBytes, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		if !bytes.Equal(eBytes, m[k]) {
			t.Fail()
		}
	}
}

func Benchmark_loadRegistrationInfo(b *testing.B) {
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	db := filepath.Join(path, "aircraft.db")
	cfg := Config{
		Directory:  path,
		BoltDbFile: db,
	}
	m, err := NewAircraftManager(logger, cfg)
	if err != nil {
		panic(err)
	}
	for i := 0; i < b.N; i++ {
		fmt.Println("Running benchmark", i)
		m.loadRegistrationInfo()
	}
}
