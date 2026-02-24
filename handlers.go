package main

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/brody192/locomotive/internal/logger"
	"github.com/brody192/locomotive/internal/railway/metrics"
	"github.com/brody192/locomotive/internal/railway/subscribe/environment_logs"
	"github.com/brody192/locomotive/internal/railway/subscribe/http_logs"
	"github.com/brody192/locomotive/internal/webhook"
)

func handleDeployLogsAsync(ctx context.Context, deployLogsProcessed *atomic.Int64, serviceLogTrack chan []environment_logs.EnvironmentLogWithMetadata) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case logs := <-serviceLogTrack:
				if serializedLogs, err := webhook.SendDeployLogsWebhook(logs); err != nil {
					attrs := []any{logger.ErrAttr(err)}

					if serializedLogs != nil {
						attrs = append(attrs, slog.String("serialized_logs", string(serializedLogs)))
					}

					logger.Stderr.Error("error sending deploy logs webhook(s)", attrs...)

					continue
				}

				deployLogsProcessed.Add(int64(len(logs)))
			}
		}
	}()
}

func handleHttpLogsAsync(ctx context.Context, httpLogsProcessed *atomic.Int64, httpLogTrack chan []http_logs.DeploymentHttpLogWithMetadata) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case logs := <-httpLogTrack:
				if serializedLogs, err := webhook.SendHttpLogsWebhook(logs); err != nil {
					attrs := []any{logger.ErrAttr(err)}

					if serializedLogs != nil {
						attrs = append(attrs, slog.String("serialized_logs", string(serializedLogs)))
					}

					logger.Stderr.Error("error sending http logs webhook(s)", attrs...)

					continue
				}

				httpLogsProcessed.Add(int64(len(logs)))
			}
		}
	}()
}

func handleMetricsAsync(ctx context.Context, metricsProcessed *atomic.Int64, metricsTrack chan []metrics.Metric) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case m := <-metricsTrack:
				if serializedMetrics, err := webhook.SendMetricsWebhook(m); err != nil {
					attrs := []any{logger.ErrAttr(err)}

					if serializedMetrics != nil {
						attrs = append(attrs, slog.String("serialized_metrics", string(serializedMetrics)))
					}

					logger.Stderr.Error("error sending metrics webhook(s)", attrs...)

					continue
				}

				metricsProcessed.Add(int64(len(m)))
			}
		}
	}()
}
