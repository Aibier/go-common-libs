package dummy

import (
	md "github.com/Aibier/go-common-libs/libs/metrics/model"
)

type DummyMetricsManager struct{}

func (dm DummyMetricsManager) CreateCounter(name string, labels ...string) md.Counter {
	return DummyCounter{}
}

func (dm DummyMetricsManager) CreateGauge(name string, labels ...string) md.Gauge {
	return DummyGauge{}
}

func NewDummyMetricsManager() (DummyMetricsManager, error) {
	return DummyMetricsManager{}, nil
}

type DummyCounter struct {
}

func (dc DummyCounter) Inc(labels map[string]string) error {
	return nil
}

type DummyGauge struct{}

func (dg DummyGauge) Set(i float64, labels map[string]string) error {
	return nil
}
