package main

import (
	v1 "k8s.io/api/core/v1"
)

type Configuration struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       Spec     `json:"spec"`
}
type Metadata struct {
	Annotations   map[string]interface{} `json:"annotations"`
	ManagedFields []interface{}          `json:"managedFields"`
	Name          string                 `json:"name"`
	Namespace     string                 `json:"namespace"`
}
type Spec struct {
	BrokerPodSpec *v1.PodSpec            `json:"brokerPodSpec"`
	Capacity      int                    `json:"capacity"`
	Protocol      map[string]interface{} `json:"protocol"`
}
