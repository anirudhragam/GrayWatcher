package main

import (
	"strings"
)

type Analyzer interface {
	Analyze(deploymentKey string, infraObs []Observation, meshObs []Observation) Verdict
}

// podNameToDeployment strips the trailing ReplicaSet hash and pod hash from a pod name.
// e.g. "frontend-7d7bc7d8d-xqk4r" → "frontend"
//      "redis-cart-6d84489d98-xtx7z" → "redis-cart"
func podNameToDeployment(podName string) string {
	parts := strings.Split(podName, "-")
	if len(parts) <= 2 {
		return podName
	}
	return strings.Join(parts[:len(parts)-2], "-")
}

// groupInfraByDeployment groups infrastructure observations by "namespace/deployment".
// The deployment name is derived from the pod name in metadata.
func groupInfraByDeployment(observations []Observation) map[string][]Observation {
	result := make(map[string][]Observation)
	for _, obs := range observations {
		if obs.ObserverType != "infrastructure" {
			continue
		}
		podName, _ := obs.Metadata["pod_name"].(string)
		namespace, _ := obs.Metadata["namespace"].(string)
		deployment := podNameToDeployment(podName)
		key := namespace + "/" + deployment
		result[key] = append(result[key], obs)
	}
	return result
}

// groupMeshByDeployment groups mesh observations by "namespace/deployment".
// The deployment name is read directly from metadata.
func groupMeshByDeployment(observations []Observation) map[string][]Observation {
	result := make(map[string][]Observation)
	for _, obs := range observations {
		if obs.ObserverType != "mesh" {
			continue
		}
		namespace, _ := obs.Metadata["namespace"].(string)
		deployment, _ := obs.Metadata["deployment"].(string)
		key := namespace + "/" + deployment
		result[key] = append(result[key], obs)
	}
	return result
}

func unionKeys(a, b map[string][]Observation) map[string]struct{} {
	keys := make(map[string]struct{})
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}
	return keys
}
