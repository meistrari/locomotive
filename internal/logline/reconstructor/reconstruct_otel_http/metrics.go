package reconstruct_otel_http

import (
	"encoding/json"
	"log/slog"
	"strconv"

	"github.com/brody192/locomotive/internal/logger"
	"github.com/brody192/locomotive/internal/railway/metrics"
)

var measurementToOtel = map[string]struct {
	Name string
	Unit string
}{
	"CPU_USAGE":               {Name: "railway.cpu.usage", Unit: "1"},
	"MEMORY_USAGE_GB":         {Name: "railway.memory.usage", Unit: "GBy"},
	"DISK_USAGE_GB":           {Name: "railway.disk.usage", Unit: "GBy"},
	"EPHEMERAL_DISK_USAGE_GB": {Name: "railway.ephemeral_disk.usage", Unit: "GBy"},
	"NETWORK_RX_GB":           {Name: "railway.network.rx", Unit: "GBy"},
	"NETWORK_TX_GB":           {Name: "railway.network.tx", Unit: "GBy"},
	"INSTANCE_COUNT":          {Name: "railway.instance.count", Unit: "1"},
}

func MetricsOtel(metricsList []metrics.Metric) ([]byte, error) {
	type resourceKey struct {
		ServiceId     string
		EnvironmentId string
	}

	type resourceGroup struct {
		tags    metrics.MetricTags
		metrics []metrics.Metric
	}

	groups := map[resourceKey]*resourceGroup{}
	groupOrder := []resourceKey{}

	for i := range metricsList {
		key := resourceKey{
			ServiceId:     metricsList[i].Tags.ServiceId.String(),
			EnvironmentId: metricsList[i].Tags.EnvironmentId.String(),
		}

		if g, ok := groups[key]; ok {
			g.metrics = append(g.metrics, metricsList[i])
		} else {
			groups[key] = &resourceGroup{
				tags:    metricsList[i].Tags,
				metrics: []metrics.Metric{metricsList[i]},
			}
			groupOrder = append(groupOrder, key)
		}
	}

	data := metricsData{
		ResourceMetrics: make([]resourceMetric, 0, len(groups)),
	}

	for _, key := range groupOrder {
		g := groups[key]

		resourceAttrs := []attribute{
			stringAttribute("service_id", g.tags.ServiceId.String()),
			stringAttribute("environment_id", g.tags.EnvironmentId.String()),
		}

		if g.tags.ProjectId.String() != "00000000-0000-0000-0000-000000000000" {
			resourceAttrs = append(resourceAttrs, stringAttribute("project_id", g.tags.ProjectId.String()))
		}

		if g.tags.Region != "" {
			resourceAttrs = append(resourceAttrs, stringAttribute("cloud_region", g.tags.Region))
		}

		if g.tags.ServiceName != "" {
			resourceAttrs = append(resourceAttrs, stringAttribute("service", g.tags.ServiceName))
		}

		if g.tags.EnvironmentName != "" {
			resourceAttrs = append(resourceAttrs, stringAttribute("env", g.tags.EnvironmentName))
		}

		if g.tags.ProjectName != "" {
			resourceAttrs = append(resourceAttrs, stringAttribute("project", g.tags.ProjectName))
		}

		var otelMetrics []metric

		for _, m := range g.metrics {
			otelInfo, ok := measurementToOtel[m.Measurement]
			if !ok {
				logger.Stdout.Warn("Measurement is not mapped and is being ignored", slog.String("measurement", m.Measurement))
				continue
			}

			points := make([]dataPoint, len(m.Values))
			for i, v := range m.Values {
				dp := dataPoint{
					TimeUnixNano: strconv.FormatInt(v.Timestamp.UnixNano(), 10),
					AsDouble:     v.Value,
					AsInt:        v.IntValue,
					Attributes:   append([]attribute{}, resourceAttrs...),
				}

				if m.Tags.DeploymentId.String() != "00000000-0000-0000-0000-000000000000" {
					dp.Attributes = append(dp.Attributes, stringAttribute("deployment_id", m.Tags.DeploymentId.String()))
				}

				if m.Tags.DeploymentInstanceId.String() != "00000000-0000-0000-0000-000000000000" {
					dp.Attributes = append(dp.Attributes, stringAttribute("deployment_instance_id", m.Tags.DeploymentInstanceId.String()))
				}

				points[i] = dp
			}

			otelMetrics = append(otelMetrics, metric{
				Name: otelInfo.Name,
				Unit: otelInfo.Unit,
				Gauge: gauge{
					DataPoints: points,
				},
			})
		}

		data.ResourceMetrics = append(data.ResourceMetrics, resourceMetric{
			Resource: resource{Attributes: resourceAttrs},
			ScopeMetrics: []scopeMetric{{
				Scope:   scope{Name: "locomotive"},
				Metrics: otelMetrics,
			}},
		})
	}

	logger.Stdout.Debug("reconstructed metrics in otel format",
		slog.Any("metrics", data),
		slog.Int("resource_metrics_count", len(data.ResourceMetrics)),
	)

	return json.Marshal(data)
}
