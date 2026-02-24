package queries

import (
	"github.com/flexstack/uuid"
)

type MetricsResponse struct {
	Metrics Metrics `json:"metrics"`
}

type Metrics []struct {
	// One of:
	// - BACKUP_USAGE_GB
	// - CPU_LIMIT
	// - CPU_USAGE
	// - CPU_USAGE_2
	// - DISK_USAGE_GB
	// - EPHEMERAL_DISK_USAGE_GB
	// - MEASUREMENT_UNSPECIFIED
	// - MEMORY_LIMIT_GB
	// - MEMORY_USAGE_GB
	// - NETWORK_RX_GB
	// - NETWORK_TX_GB
	// - UNRECOGNIZED
	Measurement string `json:"measurement"`

	Tags struct {
		DeploymentId         uuid.UUID `json:"deploymentId"`
		DeploymentInstanceId uuid.UUID `json:"deploymentInstanceId"`
		EnvironmentId        uuid.UUID `json:"environmentId"`
		ProjectId            uuid.UUID `json:"projectId"`
		Region               string    `json:"region"`
		ServiceId            uuid.UUID `json:"serviceId"`
		VolumeId             uuid.UUID `json:"volumeId"`
		VolumeInstanceId     uuid.UUID `json:"volumeInstanceId"`
	} `json:"tags"`

	Values []struct {
		Ts    int64   `json:"ts"`
		Value float64 `json:"value"`
	} `json:"values"`
}
