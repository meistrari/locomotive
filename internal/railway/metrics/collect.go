package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/brody192/locomotive/internal/logger"
	"github.com/brody192/locomotive/internal/railway"
	"github.com/brody192/locomotive/internal/railway/gql/queries"
	"github.com/flexstack/uuid"
)

var defaultMeasurements = []string{
	"CPU_USAGE",
	"MEMORY_USAGE_GB",
	"DISK_USAGE_GB",
	"EPHEMERAL_DISK_USAGE_GB",
	"NETWORK_RX_GB",
	"NETWORK_TX_GB",
}

func CollectMetrics(ctx context.Context, gqlClient *railway.GraphQLClient, metricsTrack chan<- []Metric, environmentId uuid.UUID, serviceIds []uuid.UUID, lookback time.Duration) error {
	endDate := time.Now()
	startDate := endDate.Add(-lookback)

	var allMetrics []Metric

	for _, serviceId := range serviceIds {
		resp := &queries.MetricsResponse{}

		variables := map[string]any{
			"environmentId": environmentId.String(),
			"serviceId":     serviceId.String(),
			"startDate":     startDate.Format(time.RFC3339),
			"endDate":       endDate.Format(time.RFC3339),
			"measurements":  defaultMeasurements,
		}

		logger.Stdout.Debug("querying metrics",
			slog.String("service_id", serviceId.String()),
			slog.String("environment_id", environmentId.String()),
			slog.String("start_date", startDate.Format(time.RFC3339)),
			slog.String("end_date", endDate.Format(time.RFC3339)),
			slog.Any("measurements", defaultMeasurements),
		)

		if err := gqlClient.Client.Exec(ctx, queries.MetricsQuery, resp, variables); err != nil {
			return fmt.Errorf("error querying metrics for service %s: %w", serviceId, err)
		}

		logger.Stdout.Debug("metrics query response",
			slog.String("service_id", serviceId.String()),
			slog.Int("metrics_count", len(resp.Metrics)),
		)

		for _, m := range resp.Metrics {
			metric := Metric{
				Measurement: m.Measurement,
				Tags: MetricTags{
					DeploymentId:         m.Tags.DeploymentId,
					DeploymentInstanceId: m.Tags.DeploymentInstanceId,
					EnvironmentId:        m.Tags.EnvironmentId,
					ProjectId:            m.Tags.ProjectId,
					Region:               m.Tags.Region,
					ServiceId:            m.Tags.ServiceId,
				},
				Values: make([]MetricValue, len(m.Values)),
			}

			for i, v := range m.Values {
				metric.Values[i] = MetricValue{
					Timestamp: time.UnixMilli(v.Ts),
					Value:     v.Value,
				}
			}

			allMetrics = append(allMetrics, metric)
		}
	}

	if len(allMetrics) > 0 {
		metricsTrack <- allMetrics
	}

	return nil
}
