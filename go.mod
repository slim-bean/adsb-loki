module github.com/slim-bean/adsb-loki

go 1.16

require (
	github.com/cortexproject/cortex v1.9.1-0.20210527130655-bd720c688ffa
	github.com/dimchansky/utfbom v1.1.0
	github.com/go-kit/kit v0.10.0
	github.com/gocarina/gocsv v0.0.0-20200827134620-49f5c3fa2b3e
	github.com/grafana/loki v1.6.2-0.20210709105821-1cca922e6dc0
	github.com/magefile/mage v1.11.0
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/common v0.23.0
	go.etcd.io/bbolt v1.3.5
)

replace k8s.io/client-go => k8s.io/client-go v12.0.0+incompatible