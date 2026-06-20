package metrics

import (
	"testing"

	dds "github.com/Aibier/go-common-libs/libs/metrics/ddstatsd"
	dm "github.com/Aibier/go-common-libs/libs/metrics/dummy"
)

func TestNewDummyMetricsManager(t *testing.T) {
	mm, err := NewMetricManager("dummy")
	if err != nil {
		t.Fatal(err)
	}
	switch typ := mm.(type) {
	case dm.DummyMetricsManager:
	default:
		t.Fatalf("Expected DummyMetricsManager, got %t", typ)
	}
}

func TestNewStatsdMetricsManager(t *testing.T) {
	t.Setenv("DD_STATSD_URL", "127.0.0.1:8125")
	mm, err := NewMetricManager("ddstatsd")
	if err != nil {
		t.Fatal(err)
	}
	switch typ := mm.(type) {
	case dds.DDStatsdMetricsManager:
	default:
		t.Fatalf("Expected DummyMetricsManager, got %t", typ)
	}
}

func TestNotImplementedMetricsManager(t *testing.T) {
	_, err := NewMetricManager("not_implemented")
	if err == nil {
		t.Fatal("Should have give error as Metrics Manager not implemented")
	}
}
