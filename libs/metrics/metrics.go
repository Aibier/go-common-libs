package metrics

import (
	"fmt"

	dds "github.com/Aibier/go-common-libs/libs/metrics/ddstatsd"
	dm "github.com/Aibier/go-common-libs/libs/metrics/dummy"
	md "github.com/Aibier/go-common-libs/libs/metrics/model"
)

func NewMetricManager(typ string) (md.MetricsManager, error) {
	switch typ {
	case "dummy":
		return dm.NewDummyMetricsManager()
	case "ddstatsd":
		return dds.NewDDStatsdMetricsManager()
	default:
		return nil, fmt.Errorf("Not a valid metrics type")
	}
}
