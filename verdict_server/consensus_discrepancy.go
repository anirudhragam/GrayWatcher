package main

import (
	"fmt"
	"math"
	"time"

	"gonum.org/v1/gonum/stat"
)

type ConsensusDiscrepancyAnalyzer struct{}

func (a *ConsensusDiscrepancyAnalyzer) Analyze(ns string, infraObs, meshObs []Observation) Verdict {
	if len(infraObs) == 0 || len(meshObs) == 0 {
		return Verdict{
			TargetID:    ns,
			Timestamp:   time.Now().UnixMilli(),
			VerdictType: VerdictHealthy,
			Reason:      "Insufficient data: either infra observations or mesh observations are missing.",
		}
	}

	// assuming observations arrive in order
	latestInfraObs := infraObs[len(infraObs)-1]
	latestMeshObs := meshObs[len(meshObs)-1]

	if latestInfraObs.Status == "healthy" && latestMeshObs.Status == "healthy" {
		return Verdict{
			VerdictType: VerdictHealthy,
			Confidence:  1.0,
			Reason:      "All systems normal: infrastructure healthy, service mesh healthy.",
		}
	}

	if latestInfraObs.Status == "unhealthy" && latestMeshObs.Status == "unhealthy" {
		return Verdict{
			VerdictType: VerdictHardFailure,
			Confidence:  1.0,
			Reason:      "Full outage: both infrastructure and service mesh reporting unhealthy. Pod failures are directly causing service unavailability.",
		}
	}

	// any other combination of statuses -> check grayness

	infraMetrics := []string{"cpu_utilization_percent", "memory_utilization_percent"}
	meshMetrics := []string{"p99_latency_ms", "error_rate"}

	infra_window_metrics := make(map[string][]float64, len(infraObs))
	for _, metric := range infraMetrics {
		var curr []float64
		for _, observation := range infraObs {
			val, ok := getFloat(observation.Metrics, metric)
			if ok {
				curr = append(curr, val)
			}
		}
		infra_window_metrics[metric] = curr
	}

	mesh_window_metrics := make(map[string][]float64, len(meshObs))
	for _, metric := range meshMetrics {
		var curr []float64
		for _, observation := range meshObs {
			val, ok := getFloat(observation.Metrics, metric)
			if ok {
				curr = append(curr, val)
			}
		}
		mesh_window_metrics[metric] = curr
	}

	A_cpu := anomalyDegree(infra_window_metrics["cpu_utilization_percent"])
	A_mem := anomalyDegree(infra_window_metrics["memory_utilization_percent"])
	A_p99 := anomalyDegree(mesh_window_metrics["p99_latency_ms"])
	A_error := anomalyDegree(mesh_window_metrics["error_rate"])

	infraAnomaly := mean(A_cpu, A_mem)
	meshAnomaly := mean(A_p99, A_error)

	latestCPU, _ := getFloat(latestInfraObs.Metrics, "cpu_utilization_percent")
	latestMem, _ := getFloat(latestInfraObs.Metrics, "memory_utilization_percent")
	latestP99, _ := getFloat(latestMeshObs.Metrics, "p99_latency_ms")
	latestErrorRate, _ := getFloat(latestMeshObs.Metrics, "error_rate")

	// correlation degradation guard
	// checks both anomalies are statistical outliers (>2*mean)
	const correlatedThreshold = 2.0
	finding, suggestion := diagnose(infraAnomaly, meshAnomaly, latestInfraObs.Status, latestMeshObs.Status, latestCPU, latestMem, latestP99, latestErrorRate)

	if infraAnomaly > correlatedThreshold && meshAnomaly > correlatedThreshold {
		return Verdict{
			TargetID:    ns,
			Timestamp:   time.Now().UnixMilli(),
			VerdictType: VerdictHardFailure,
			Confidence:  1.0,
			Reason:      fmt.Sprintf("Infrastructure exhaustion causing service degradation. %s Suggestion: %s (infraAnomaly=%.2f meshAnomaly=%.2f)", finding, suggestion, infraAnomaly, meshAnomaly),
		}
	}

	residual := meshAnomaly - infraAnomaly
	grayness := math.Tanh(math.Max(residual, 0))

	return Verdict{
		TargetID:    ns,
		Timestamp:   time.Now().UnixMilli(),
		VerdictType: VerdictGrayFailure,
		Confidence:  grayness,
		Reason:      fmt.Sprintf("%s Suggestion: %s (grayness=%.4f)", finding, suggestion, grayness),
		Indicators: []Indicator{
			{Component: "infrastructure", Name: "anomaly_degree", Signal: fmt.Sprintf("%.4f", infraAnomaly)},
			{Component: "mesh", Name: "anomaly_degree", Signal: fmt.Sprintf("%.4f", meshAnomaly)},
		},
	}
}

func getFloat(m map[string]interface{}, key string) (float64, bool) {
	if val, ok := m[key]; ok {
		f, ok := val.(float64)
		return f, ok
	}
	return 0.0, false
}

func mean(a, b float64) float64 {
	return a + (b-a)/2.0
}

func anomalyDegree(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	mean := stat.Mean(values, nil)
	stdDev := max(stat.StdDev(values, nil), 1e-9)

	anomaly := math.Abs((values[len(values)-1] - mean) / stdDev)
	return anomaly
}

func diagnose(infraAnomaly, meshAnomaly float64, infraStatus, meshStatus string, latestCPU, latestMem, latestP99, latestErrorRate float64) (finding string, suggestion string) {
	switch {
	case infraStatus == "unhealthy" && meshStatus == "healthy":
		return "Pod is restarting or failed but traffic is still being served — likely recovering quickly.",
			"Check restart loops and liveness/readiness probe configuration."

	case infraStatus == "healthy" && meshStatus == "unhealthy":
		return "Service is failing despite healthy pods — infrastructure is not the cause.",
			"Check recent deployments, application logs, and upstream dependencies."

	case infraAnomaly > 1.5 && meshAnomaly <= 1.0:
		return fmt.Sprintf("Infrastructure under pressure (CPU=%.1f%%, mem=%.1f%%) but traffic is unaffected yet — early warning.", latestCPU, latestMem),
			"Consider scaling the deployment or raising CPU/memory limits before this impacts users."

	case infraAnomaly <= 1.0 && meshAnomaly > 1.5:
		return fmt.Sprintf("Application-layer degradation with no infrastructure cause: p99=%.0fms, error_rate=%.1f%%. Classic gray failure.", latestP99, latestErrorRate*100),
			"Inspect application logs, check for upstream dependency failures or bad deploys."

	default:
		return fmt.Sprintf("Partial degradation detected: infra anomaly=%.2f, mesh anomaly=%.2f.", infraAnomaly, meshAnomaly),
			"Monitor for escalation. Check both pod health and service mesh metrics."
	}
}
