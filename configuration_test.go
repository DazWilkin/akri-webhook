package main

import (
	"encoding/json"
	"testing"
)

var configuration = `
{
	"apiVersion": "akri.sh/v0",
	"kind": "Configuration",
	"metadata": {
		"annotations": {
			"kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"akri.sh/v0\",\"kind\":\"Configuration\",\"metadata\":{\"annotations\":{},\"name\":\"zeroconf\",\"namespace\":\"default\"},\"spec\":{\"brokerPodSpec\":{\"containers\":[{\"image\":\"ghcr.io/dazwilkin/zeroconf-broker@sha256:69810b622d37d0a9a544955d4d4c53f16fec6b8d32a111740f4503dcc164fcf0\",\"name\":\"zeroconf-broker\",\"resources\":{\"limits\":{\"{{PLACEHOLDER}}\":\"1\"}}}],\"imagePullSecrets\":[{\"name\":\"ghcr\"}]},\"capacity\":1,\"protocol\":{\"zeroconf\":{\"kind\":\"_rust._tcp\",\"port\":8888,\"txtRecords\":{\"component\":\"avahi-publish\",\"project\":\"akri\",\"protocol\":\"zeroconf\"}}}}}\n"
		},
		"creationTimestamp": "2020-12-23T20:20:43Z",
		"generation": 1,
		"managedFields": [{
			"apiVersion": "akri.sh/v0",
			"fieldsType": "FieldsV1",
			"fieldsV1": {
				"f:metadata": {
					"f:annotations": {
						".": {},
						"f:kubectl.kubernetes.io/last-applied-configuration": {}
					}
				},
				"f:spec": {
					".": {},
					"f:brokerPodSpec": {
						".": {},
						"f:containers": {},
						"f:imagePullSecrets": {}
					},
					"f:capacity": {},
					"f:protocol": {
						".": {},
						"f:zeroconf": {
							".": {},
							"f:kind": {},
							"f:port": {},
							"f:txtRecords": {
								".": {},
								"f:component": {},
								"f:project": {},
								"f:protocol": {}
							}
						}
					}
				}
			},
			"manager": "kubectl",
			"operation": "Update",
			"time": "2020-12-23T20:20:43Z"
		}],
		"name": "zeroconf",
		"namespace": "default",
		"uid": "8a8b372f-a301-4afb-9603-b9a6f9573c2d"
	},
	"spec": {
		"brokerPodSpec": {
			"containers": [{
				"image": "ghcr.io/dazwilkin/zeroconf-broker@sha256:69810b622d37d0a9a544955d4d4c53f16fec6b8d32a111740f4503dcc164fcf0",
				"name": "zeroconf-broker",
				"resources": {
					"limits": {
						"{{PLACEHOLDER}}": "1"
					}
				}
			}],
			"imagePullSecrets": [{
				"name": "ghcr"
			}]
		},
		"capacity": 1,
		"protocol": {
			"zeroconf": {
				"kind": "_rust._tcp",
				"port": 8888,
				"txtRecords": {
					"component": "avahi-publish",
					"project": "akri",
					"protocol": "zeroconf"
				}
			}
		}
	}
}
`

func TestConfiguration(t *testing.T) {
	got := &Configuration{}
	if err := json.Unmarshal([]byte(configuration), &got); err != nil {
		panic(err)
	}
	want := &Configuration{}
	if got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}
