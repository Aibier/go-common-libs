package ddstatsd

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/DataDog/datadog-go/v5/statsd"
)

func TestNewDDStatsdMetricsManagerStatsd(t *testing.T) {
	t.Setenv("DD_STATSD_URL", "127.0.0.1:8125")
	mm, err := NewDDStatsdMetricsManager()
	if err != nil {
		t.Fatal(err)
	}
	switch typ := mm.statsdc.(type) {
	case *statsd.Client:
	default:
		t.Fatalf("Expected statsd.Client, got %t", typ)
	}
}

func TestNewDDStatsdMetricsManagerNOOP(t *testing.T) {
	t.Setenv("DD_STATSD_URL", "NOOP")
	mm, err := NewDDStatsdMetricsManager()
	if err != nil {
		t.Fatal(err)
	}
	switch typ := mm.statsdc.(type) {
	case *statsd.NoOpClient:
	default:
		t.Fatalf("Expected statsd.NoOpClient, got %t", typ)
	}
}

func TestCreateCounter(t *testing.T) {
	t.Setenv("DD_STATSD_URL", "NOOP")
	mm, err := NewDDStatsdMetricsManager()
	if err != nil {
		t.Fatal(err)
	}
	ctmetrics := mm.CreateCounter("test_counter")
	counter, ok := ctmetrics.(DDStatsdCounter)
	if !ok {
		t.Fatal(errors.New("Expected counter type"))
	}
	if counter.name != "test_counter" {
		t.Error(fmt.Errorf("Expected name as test_counter got: %s", counter.name))
	}
}

func TestCreateGauge(t *testing.T) {
	t.Setenv("DD_STATSD_URL", "NOOP")
	mm, err := NewDDStatsdMetricsManager()
	if err != nil {
		t.Fatal(err)
	}
	ctmetrics := mm.CreateGauge("test_gauge")
	counter, ok := ctmetrics.(DDStatsdGauge)
	if !ok {
		t.Fatal(errors.New("Expected gauge type"))
	}
	if counter.name != "test_gauge" {
		t.Error(fmt.Errorf("Expected name as test_gauge got: %s", counter.name))
	}
}

func TestIncCounter(t *testing.T) {
	t.Setenv("DD_STATSD_URL", "NOOP")
	mm, err := NewDDStatsdMetricsManager()
	if err != nil {
		t.Fatal(err)
	}
	ctmetrics := mm.CreateCounter("test_counter")
	err = ctmetrics.Inc(map[string]string{"one": "some"})
	if err != nil {
		t.Fatal(errors.New("Inc counter failed"))
	}
}

func TestSetGauge(t *testing.T) {
	t.Setenv("DD_STATSD_URL", "NOOP")
	mm, err := NewDDStatsdMetricsManager()
	if err != nil {
		t.Fatal(err)
	}
	ctmetrics := mm.CreateGauge("test_gauge")
	err = ctmetrics.Set(3.0, map[string]string{"one": "some"})
	if err != nil {
		t.Fatal(errors.New("Set gauge failed"))
	}
}

func TestGetTags(t *testing.T) {
	labels := map[string]string{
		"abc": "1",
		"def": "2",
	}
	tags := getTags(labels)
	if !slices.Contains(tags, "{abc:1}") {
		t.Error("Expected to find {abc:1}")
	}
	if !slices.Contains(tags, "{def:2}") {
		t.Error("Expected to find {def:1}")
	}
	if l := len(tags); l != 2 {
		t.Errorf("Expected length to be 2, was %d", l)
	}
}
