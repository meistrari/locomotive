package reconstruct_otel_http

import (
	"encoding/json"
	"log/slog"
	"strconv"

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
			stringAttribute("service.instance.id", g.tags.ServiceId.String()),
			stringAttribute("deployment.environment.id", g.tags.EnvironmentId.String()),
		}

		if g.tags.ProjectId.String() != "00000000-0000-0000-0000-000000000000" {
			resourceAttrs = append(resourceAttrs, stringAttribute("railway.project.id", g.tags.ProjectId.String()))
		}

		if g.tags.Region != "" {
			resourceAttrs = append(resourceAttrs, stringAttribute("cloud.region", g.tags.Region))
		}

		var otelMetrics []metric

		for _, m := range g.metrics {
			otelInfo, ok := measurementToOtel[m.Measurement]
			if !ok {
				slog.Warn("Measurement is not mapped and is being ignored", slog.String("measurement", m.Measurement))
				continue
			}

			points := make([]dataPoint, len(m.Values))
			for i, v := range m.Values {
				dp := dataPoint{
					TimeUnixNano: strconv.FormatInt(v.Timestamp.UnixNano(), 10),
					AsDouble:     v.Value,
				}

				if m.Tags.DeploymentId.String() != "00000000-0000-0000-0000-000000000000" {
					dp.Attributes = append(dp.Attributes, stringAttribute("railway.deployment.id", m.Tags.DeploymentId.String()))
				}

				if m.Tags.DeploymentInstanceId.String() != "00000000-0000-0000-0000-000000000000" {
					dp.Attributes = append(dp.Attributes, stringAttribute("railway.deployment_instance.id", m.Tags.DeploymentInstanceId.String()))
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

	return json.Marshal(data)
}
