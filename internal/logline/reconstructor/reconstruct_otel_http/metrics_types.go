package reconstruct_otel_http

type metricsData struct {
	ResourceMetrics []resourceMetric `json:"resourceMetrics"`
}

type resourceMetric struct {
	Resource     resource      `json:"resource"`
	ScopeMetrics []scopeMetric `json:"scopeMetrics"`
}

type scopeMetric struct {
	Scope   scope    `json:"scope"`
	Metrics []metric `json:"metrics"`
}

type metric struct {
	Name  string `json:"name"`
	Unit  string `json:"unit"`
	Gauge gauge  `json:"gauge"`
}

type gauge struct {
	DataPoints []dataPoint `json:"dataPoints"`
}

type dataPoint struct {
	TimeUnixNano string      `json:"timeUnixNano"`
	AsDouble     float64     `json:"asDouble"`
	Attributes   []attribute `json:"attributes,omitempty"`
}
