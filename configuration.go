package main

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Configuration struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	Metadata   *metav1.ObjectMeta `json:"metadata"`
	Spec       Spec               `json:"spec"`
}
type Spec struct {
	BrokerPodSpec            *v1.PodSpec            `json:"brokerPodSpec,omitempty"`
	Capacity                 int                    `json:"capacity"`
	ConfigurationServiceSepc *v1.ServiceSpec        `json:"configurationServiceSpec,omitempty"`
	InstanceServiceSpec      *v1.ServiceSpec        `json:"instanceServiceSpec,omitempty"`
	Properties               map[string]string      `json:"properties,omitempty"`
	Protocol                 map[string]interface{} `json:"protocol"`
	Units                    string                 `json:"units"`
}
