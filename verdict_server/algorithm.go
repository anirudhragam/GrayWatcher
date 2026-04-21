package main

import (
	"strings"
)

type Analyzer interface {
    Analyze(namespace string, infraObs []Observation, meshObs []Observation) Verdict
}

func extractNamespace(targetID string) string   {
    return strings.SplitN(targetID, "/", 3)[1] 
}

func groupByNamespace(observations []Observation) map[string][]Observation {
    ns_obs := make(map[string][]Observation)
    for _, observation := range observations {
        namespace := extractNamespace(observation.TargetID)
        ns_obs[namespace] = append(ns_obs[namespace], observation)
    }
    return ns_obs
}

func splitByType(grouped map[string][]Observation) (infra, mesh map[string][]Observation){
    infra_obs := make(map[string][]Observation)
    mesh_obs := make(map[string][]Observation)

    for ns, observations := range grouped {
        for _, observation := range observations {
            if observation.ObserverType == "infrastructure" {
                infra_obs[ns] = append(infra_obs[ns], observation)
            } else {
                mesh_obs[ns] = append(mesh_obs[ns], observation)
            }
        }
    }
    return infra_obs, mesh_obs
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
