module adsb-loki

go 1.16

require (
	github.com/cortexproject/cortex v1.4.1-0.20201022071705-85942c5703cf
	github.com/dimchansky/utfbom v1.1.0
	github.com/go-kit/kit v0.10.0
	github.com/gocarina/gocsv v0.0.0-20200827134620-49f5c3fa2b3e
	github.com/grafana/loki v1.6.2-0.20201026154740-6978ee5d7387
	github.com/magefile/mage v1.11.0
	github.com/prometheus/common v0.14.0
)

// Override reference that causes an error from Go proxy - see https://github.com/golang/go/issues/33558
replace k8s.io/client-go => k8s.io/client-go v0.18.3
