module adsb-loki

go 1.15

require (
	github.com/cortexproject/cortex v1.2.1-0.20200803161316-7014ff11ed70
	github.com/dimchansky/utfbom v1.1.0
	github.com/go-kit/kit v0.10.0
	github.com/gocarina/gocsv v0.0.0-20200827134620-49f5c3fa2b3e
	github.com/grafana/loki v1.6.1
	github.com/prometheus/common v0.10.0
	github.com/wz2b/dump1090-go-client v0.0.0-20200918202426-ba9951d82abb
)

// Override reference that causes an error from Go proxy - see https://github.com/golang/go/issues/33558
replace k8s.io/client-go => k8s.io/client-go v0.18.3
