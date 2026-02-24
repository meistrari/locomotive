package metrics

import (
	"time"

	"github.com/flexstack/uuid"
)

type Metric struct {
	Measurement string
	Tags        MetricTags
	Values      []MetricValue
}

type MetricTags struct {
	DeploymentId         uuid.UUID
	DeploymentInstanceId uuid.UUID
	EnvironmentId        uuid.UUID
	ProjectId            uuid.UUID
	Region               string
	ServiceId            uuid.UUID
	ServiceName          string
	EnvironmentName      string
	ProjectName          string
}

type MetricValue struct {
	Timestamp time.Time
	Value     float64
	IntValue  int64
}
