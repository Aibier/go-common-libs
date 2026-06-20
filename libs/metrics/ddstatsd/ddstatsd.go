package ddstatsd

import (
	"fmt"
	"os"

	"github.com/DataDog/datadog-go/v5/statsd"
	md "github.com/Aibier/go-common-libs/libs/metrics/model"
)

type DDStatsdMetricsManager struct {
	statsdc statsd.ClientInterface
}

func (dm DDStatsdMetricsManager) CreateCounter(name string, labels ...string) md.Counter {
	return DDStatsdCounter{
		name:    name,
		statsdc: dm.statsdc,
	}
}

func (dm DDStatsdMetricsManager) CreateGauge(name string, labels ...string) md.Gauge {
	return DDStatsdGauge{
		name:    name,
		statsdc: dm.statsdc,
	}
}

func NewDDStatsdMetricsManager() (DDStatsdMetricsManager, error) {
	statsdUrl := os.Getenv("DD_STATSD_URL")
	var statsdc statsd.ClientInterface
	if statsdUrl == "NOOP" {
		statsdc = &statsd.NoOpClient{}
	} else {
		var err error
		statsdc, err = statsd.New(statsdUrl)
		if err != nil {
			return DDStatsdMetricsManager{}, err
		}
	}
	return DDStatsdMetricsManager{
		statsdc: statsdc,
	}, nil
}

type DDStatsdCounter struct {
	name    string
	statsdc statsd.ClientInterface
}

func (dc DDStatsdCounter) Inc(labels map[string]string) error {
	return dc.statsdc.Incr(dc.name, getTags(labels), 1)
}

type DDStatsdGauge struct {
	name    string
	statsdc statsd.ClientInterface
}

func (dg DDStatsdGauge) Set(i float64, labels map[string]string) error {
	return dg.statsdc.Gauge(dg.name, i, getTags(labels), 1)
}

func getTags(labels map[string]string) []string {
	var tags []string
	for k, v := range labels {
		tag := fmt.Sprintf("{%s:%s}", k, v)
		tags = append(tags, tag)
	}
	return tags
}
