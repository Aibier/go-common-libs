package model

type MetricsManager interface {
	CreateCounter(name string, labels ...string) Counter
	CreateGauge(name string, labels ...string) Gauge
}

type Counter interface {
	Inc(labels map[string]string) error
}

type Gauge interface {
	Set(i float64, labels map[string]string) error
}
