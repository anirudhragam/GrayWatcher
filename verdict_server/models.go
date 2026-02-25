package main

const (
	VerdictHealthy     = "healthy"
	VerdictGrayFailure = "gray_failure"
	VerdictHardFailure = "hard_failure"
)

type InfraObservation struct {
	ObserverID   string        `json:"observer_id"`
	ObserverType string        `json:"observer_type"`
	TargetID     string        `json:"target_id"`
	Status       string        `json:"status"`
	Metrics      InfraMetrics  `json:"metrics"`
	Timestamp    int64         `json:"timestamp"`
	Metadata     InfraMetadata `json:"metadata"`
}

type InfraMetadata struct {
	NodeName  string `json:"node"`
	PodName   string `json:"pod_name"`
	Namespace string `json:"namespace"`
}

type InfraMetrics struct {
	Phase           string          `json:"phase"`
	ReadyContainers int             `json:"ready_containers"`
	TotalContainers int             `json:"total_containers"`
	RestartCount    int             `json:"restart_count"`
	Conditions      map[string]bool `json:"conditions"`
}

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
