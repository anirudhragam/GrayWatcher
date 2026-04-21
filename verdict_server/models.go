package main

const (
	VerdictHealthy     = "healthy"
	VerdictGrayFailure = "gray_failure"
	VerdictHardFailure = "hard_failure"
)

type Observation struct {
	ObserverID   string                 `json:"observer_id"`
	ObserverType string                 `json:"observer_type"`
	TargetID     string                 `json:"target_id"`
	Status       string                 `json:"status"`
	Confidence   float64                `json:"confidence"`
	Metrics      map[string]interface{} `json:"metrics"`
	Timestamp    int64                  `json:"timestamp"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// Infrastructure Observation structs
type InfraMetadata struct {
	NodeName  string `json:"node"`
	PodName   string `json:"pod_name"`
	Namespace string `json:"namespace"`
}

type InfraMetrics struct {
	Phase                    string          `json:"phase"`
	ReadyContainers          int             `json:"ready_containers"`
	TotalContainers          int             `json:"total_containers"`
	RestartCount             int             `json:"restart_count"`
	Conditions               map[string]bool `json:"conditions"`
	CPUUsageMillicores       int             `json:"cpu_usage_millicores"`
	MemoryUsageBytes         int             `json:"memory_usage_bytes"`
	CPUUtilisationPercent    float64         `json:"cpu_utilization_percent"`
	MemoryUtilisationPercent float64         `json:"memory_utilization_percent"`
}

// Mesh Observations structs
type MeshMetadata struct {
	Deployment string `json:"deployment"`
	Namespace  string `json:"namespace"`
}

type MeshMetrics struct {
	SuccessRate  float64 `json:"success_rate"`
	P99LatencyMs float64 `json:"p99_latency_ms"`
	ErrorRate    float64 `json:"error_rate"`
	RequestRate  float64 `json:"request_rate"`
}

// Verdict structs

type Indicator struct {
	Component string `json:"component"`
	Name      string `json:"name"`
	Signal    string `json:"signal"`
	Details   string `json:"details"`
}

type Verdict struct {
	TargetID    string      `json:"target_id"`
	Timestamp   int64       `json:"timestamp"`
	VerdictType string      `json:"verdict_type"`
	Confidence  float64     `json:"confidence"`
	Reason      string      `json:"reason"`
	Indicators  []Indicator `json:"indicators"`
}
