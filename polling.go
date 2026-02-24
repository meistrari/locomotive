package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/brody192/locomotive/internal/logger"
	"github.com/brody192/locomotive/internal/railway"
	"github.com/brody192/locomotive/internal/railway/metrics"
	"github.com/flexstack/uuid"
	"github.com/sethvargo/go-retry"
)

func startCollectingMetrics(ctx context.Context, gqlClient *railway.GraphQLClient, metricsTrack chan []metrics.Metric, environmentId uuid.UUID, serviceIds []uuid.UUID, metricCollectionInterval time.Duration) error {
	ticker := time.NewTicker(metricCollectionInterval)

	collectMetrics(ctx, gqlClient, metricsTrack, environmentId, serviceIds, metricCollectionInterval)

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return nil
		case <-ticker.C:
			collectMetrics(ctx, gqlClient, metricsTrack, environmentId, serviceIds, metricCollectionInterval)
		}
	}
}

func collectMetrics(ctx context.Context, gqlClient *railway.GraphQLClient, metricsTrack chan []metrics.Metric, environmentId uuid.UUID, serviceIds []uuid.UUID, lookback time.Duration) error {
	b := retry.NewFibonacci(100 * time.Millisecond)
	b = retry.WithCappedDuration((5 * time.Second), b)
	b = retry.WithMaxRetries(10, b)

	if err := retry.Do(ctx, b, func(ctx context.Context) error {
		if err := metrics.CollectMetrics(ctx, gqlClient, metricsTrack, environmentId, serviceIds, lookback); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}

			return retry.RetryableError(err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("error collecting metrics: %w", err)
	}

	logger.Stdout.Debug("metrics collection ended")

	return nil
}
