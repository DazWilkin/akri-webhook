package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"k8s.io/client-go/util/jsonpath"
)

// Example Configuration (Zeroconf)
var good = `
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

// Possible to have multiple containers but (!?) only one with `{{PLACEHOLDER}}`
var mult = `
{
	"apiVersion": "akri.sh/v0",
	"kind": "Configuration",
	"metadata": {
		"generation": 1,
		"name": "test"
	},
	"spec": {
		"brokerPodSpec": {
			"containers": [{
				"image": "image",
				"name": "name",
				"resources": {
					"limits": {
						"{{PLACEHOLDER}}": "3"
					}
				}
			}, {
				"image": "sidecar-1",
				"name": "name",
				"resources": {
					"requests": {
						"cpu": "250m",
						"memory": "64Mi"
					},
					"limits": {
						"cpu": "500m",
						"memory": "128Mi"
					}
				}
			}, {
				"image": "sidecar-2",
				"name": "name",
				"resources": {
					"limits": {
						"bar": "1"
					}
				}
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

// My incorrect configuration
var bad = `
{
	"apiVersion": "akri.sh/v0",
	"kind": "Configuration",
	"metadata": {
	   "name": "zeroconf"
	},
	"spec": {
	   "protocol": {
		  "zeroconf": {
			 "kind": "_rust._tcp",
			 "port": 8888,
			 "txtRecords": {
				"project": "akri",
				"protocol": "zeroconf",
				"component": "avahi-publish"
			 }
		  }
	   },
	   "capacity": 1,
	   "brokerPodSpec": {
		  "imagePullSecrets": [
			 {
				"name": "ghcr"
			 }
		  ],
		  "containers": [
			 {
				"name": "zeroconf-broker",
				"image": "ghcr.io/dazwilkin/zeroconf-broker@sha256:993e5b8d...."
			 }
		  ],
		  "resources": "<----------------------------- INCORRECTLY INDENTED AFTER EDIT\nlimits: \n  \"{{PLACEHOLDER}}\": \"1\""
	   }
	}
 }
`

const template = "{.spec.brokerPodSpec.containers[*].resources.limits}"

func TestUnmarshalConfiguration(t *testing.T) {
	got := &Configuration{}
	if err := json.Unmarshal([]byte(good), &got); err != nil {
		t.Error(err)
	}
}
func TestJSONPath(t *testing.T) {
	var v interface{}
	if err := json.Unmarshal([]byte(mult), &v); err != nil {
		t.Error(err)
	}

	j := jsonpath.New("limits")
	j.AllowMissingKeys(false)
	if err := j.Parse(template); err != nil {
		t.Error(err)
	}

	buf := new(bytes.Buffer)
	if err := j.Execute(buf, v); err != nil {
		t.Error(err)
	}
	got := buf.String()
	want := "{{PLACEHOLDER}}"
	if !strings.Contains(got, want) {
		t.Errorf("[test] got: %s; want; %s", got, want)
	}
}
