package promutil

import "github.com/prometheus/client_golang/prometheus"

type MetricDesc struct {
	fqName string
	help   string
}

func NewMetricDesc(opts prometheus.Opts) *MetricDesc {
	return &MetricDesc{
		fqName: prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
		help:   opts.Help,
	}
}

func (d *MetricDesc) Desc(labels prometheus.Labels) *prometheus.Desc {
	return prometheus.NewDesc(d.fqName, d.help, nil, labels)
}

func (d *MetricDesc) Counter(value float64, labels prometheus.Labels) prometheus.Metric {
	return prometheus.MustNewConstMetric(d.Desc(labels), prometheus.CounterValue, value)
}

func (d *MetricDesc) Gauge(value float64, labels prometheus.Labels) prometheus.Metric {
	return prometheus.MustNewConstMetric(d.Desc(labels), prometheus.GaugeValue, value)
}

func (d *MetricDesc) GaugeBool(value bool, labels prometheus.Labels) prometheus.Metric {
	v := float64(0)
	if value {
		v = 1
	}
	return d.Gauge(v, labels)
}
