package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/brody192/locomotive/internal/logger"
	"github.com/brody192/locomotive/internal/railway"
	"github.com/brody192/locomotive/internal/railway/gql/queries"
	"github.com/flexstack/uuid"
	"github.com/hasura/go-graphql-client"
)

var metadataEnvironmentCache = cache.New[uuid.UUID, map[uuid.UUID]string]()

func getMetadataMapForEnvironment(ctx context.Context, g *graphql.Client, environmentId uuid.UUID) (map[uuid.UUID]string, error) {
	metadataMap, ok := metadataEnvironmentCache.Get(environmentId)
	if ok {
		return metadataMap, nil
	}

	if g == nil {
		return nil, errors.New("client is nil")
	}

	environment := &queries.EnvironmentData{}

	variables := map[string]any{
		"id": environmentId,
	}

	if err := g.Exec(ctx, queries.EnvironmentQuery, &environment, variables); err != nil {
		return nil, err
	}

	project := &queries.ProjectData{}

	variables = map[string]any{
		"id": environment.Environment.ProjectID,
	}

	if err := g.Exec(ctx, queries.ProjectQuery, &project, variables); err != nil {
		return nil, err
	}

	idToNameMap := make(map[uuid.UUID]string)

	for _, e := range project.Project.Environments.Edges {
		idToNameMap[e.Node.ID] = e.Node.Name
	}

	for _, s := range project.Project.Services.Edges {
		idToNameMap[s.Node.ID] = s.Node.Name
	}

	idToNameMap[project.Project.ID] = project.Project.Name

	metadataEnvironmentCache.Set(environmentId, idToNameMap, cache.WithExpiration(10*time.Minute))

	return idToNameMap, nil
}

var defaultMeasurements = []string{
	"CPU_USAGE",
	"MEMORY_USAGE_GB",
	"DISK_USAGE_GB",
	"EPHEMERAL_DISK_USAGE_GB",
	"NETWORK_RX_GB",
	"NETWORK_TX_GB",
}

func CollectMetrics(ctx context.Context, gqlClient *railway.GraphQLClient, metricsTrack chan<- []Metric, environmentId uuid.UUID, serviceIds []uuid.UUID, lookback time.Duration) error {
	metadataMap, err := getMetadataMapForEnvironment(ctx, gqlClient.Client, environmentId)
	if err != nil {
		return fmt.Errorf("error getting metadata map: %w", err)
	}

	startDate := time.Now().Add(-lookback)

	var allMetrics []Metric

	for _, serviceId := range serviceIds {
		resp := &queries.MetricsResponse{}

		variables := map[string]any{
			"environmentId": environmentId.String(),
			"serviceId":     serviceId.String(),
			"startDate":     startDate.Format(time.RFC3339),
			"measurements":  defaultMeasurements,
		}

		logger.Stdout.Debug("querying metrics",
			slog.String("serviceId", serviceId.String()),
			slog.String("environmentId", environmentId.String()),
			slog.String("startDate", startDate.Format(time.RFC3339)),
			slog.Any("measurements", defaultMeasurements),
		)

		if err := gqlClient.Client.Exec(ctx, queries.MetricsQuery, resp, variables); err != nil {
			return fmt.Errorf("error querying metrics for service %s: %w", serviceId, err)
		}

		logger.Stdout.Debug("metrics query response",
			slog.String("service_id", serviceId.String()),
			slog.Int("metrics_count", len(resp.Metrics)),
		)

		// Group instances by deployment to avoid counting the same replica twice
		// across deployments. The max across deployments reflects the true peak
		// during any transition period, and self-corrects once old deployments
		// fall out of the lookback window.
		deploymentInstances := make(map[uuid.UUID]map[uuid.UUID]struct{})

		for _, m := range resp.Metrics {
			serviceName, _ := metadataMap[m.Tags.ServiceId]
			environmentName, _ := metadataMap[m.Tags.EnvironmentId]
			projectName, _ := metadataMap[m.Tags.ProjectId]

			if (m.Tags.DeploymentInstanceId != uuid.UUID{}) {
				deploymentId := m.Tags.DeploymentId
				if deploymentInstances[deploymentId] == nil {
					deploymentInstances[deploymentId] = make(map[uuid.UUID]struct{})
				}
				deploymentInstances[deploymentId][m.Tags.DeploymentInstanceId] = struct{}{}
			}

			metric := Metric{
				Measurement: m.Measurement,
				Tags: MetricTags{
					DeploymentId:         m.Tags.DeploymentId,
					DeploymentInstanceId: m.Tags.DeploymentInstanceId,
					EnvironmentId:        m.Tags.EnvironmentId,
					ProjectId:            m.Tags.ProjectId,
					Region:               m.Tags.Region,
					ServiceId:            m.Tags.ServiceId,
					ServiceName:          serviceName,
					EnvironmentName:      environmentName,
					ProjectName:          projectName,
				},
				Values: make([]MetricValue, len(m.Values)),
			}

			for i, v := range m.Values {
				metric.Values[i] = MetricValue{
					Timestamp: time.Unix(v.Ts, 0),
					Value:     v.Value,
				}
			}

			allMetrics = append(allMetrics, metric)
		}

		var maxInstanceCount int64
		for _, instances := range deploymentInstances {
			if count := int64(len(instances)); count > maxInstanceCount {
				maxInstanceCount = count
			}
		}

		if maxInstanceCount > 0 {
			serviceName, _ := metadataMap[serviceId]
			environmentName, _ := metadataMap[environmentId]

			allMetrics = append(allMetrics, Metric{
				Measurement: "INSTANCE_COUNT",
				Tags: MetricTags{
					EnvironmentId:   environmentId,
					ServiceId:       serviceId,
					ServiceName:     serviceName,
					EnvironmentName: environmentName,
					ProjectName:     metadataMap[resp.Metrics[0].Tags.ProjectId],
					ProjectId:       resp.Metrics[0].Tags.ProjectId,
					Region:          resp.Metrics[0].Tags.Region,
				},
				Values: []MetricValue{
					{
						Timestamp: time.Now(),
						IntValue:  maxInstanceCount,
					},
				},
			})
		}
	}

	if len(allMetrics) > 0 {
		metricsTrack <- allMetrics
	}

	return nil
}
