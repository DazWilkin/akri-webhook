/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	crtFile = flag.String("tls-crt-file", "", "TLS certificate file")
	keyFile = flag.String("tls-key-file", "", "TLS key file")
	port    = flag.Int("port", 0, "Webhook Port")
)

func invalidConfiguration(uid types.UID) func(message string) *v1beta1.AdmissionResponse {
	return func(message string) *v1beta1.AdmissionResponse {
		return &v1beta1.AdmissionResponse{
			UID:     uid,
			Allowed: false,
			Result: &metav1.Status{
				Message: message,
			},
		}
	}
}
func validateConfiguration(rqst v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	invalid := invalidConfiguration(rqst.Request.UID)

	// See: https://github.com/kubernetes/apimachinery/issues/102
	raw := rqst.Request.Object.Raw

	if len(raw) == 0 {
		klog.Error("[serve] AdmissionReview contains no data")
		return invalid("AdmissionReview contains no data")
	}

	configuration := &Configuration{}

	if err := json.Unmarshal(raw, configuration); err != nil {
		klog.Errorf("[serve] Unable to unmarshal akri.sh/v0/Configuration: %v", err)
		return invalid(err.Error())
	}

	// Check container resources [required]
	if len(configuration.Spec.BrokerPodSpec.Containers) == 0 {
		klog.Error("[serve] No containers in akri.sh/v0/Configuration")
		return invalid("Configuration has no containers")
	}
	for _, c := range configuration.Spec.BrokerPodSpec.Containers {
		r := c.Resources
		klog.V(2).Infof("[serve] Container: %+v", r)
		if len(r.Limits) == 0 {
			klog.Errorf("[serve] Container [%s] has no `resources.limits` in akri.sh/v0/Configuration: %v", c.Name)
			return invalid("Configuration does not include `resources.limits`")
		}
		for k := range r.Limits {
			if k == "" {
				klog.Errorf("[serve] Expect key in `spec.containers[\"%s\"].resources.limits`: %s", k)
				return invalid(fmt.Sprintf("Expect key in `spec.containers[\"%s\"].resources.limits`: %s", c.Name, k))
			}
		}
		if len(r.Requests) == 0 {
			klog.Errorf("[serve] Container [%s] has no `resources.requests` in akri.sh/v0/Configuration: %v", c.Name)
			return invalid("Configuration does not include `resources.requests`")
		}
		for k := range r.Requests {
			if k == "" {
				klog.Errorf("[serve] spec.containers[\"%s\"].resources.requests: %s", k)
				return invalid(fmt.Sprintf("Expect key in `spec.containers[\"%s\"].resources.requests`: %s", c.Name, k))
			}
		}
	}

	// Otherwise, we're good!
	return &v1beta1.AdmissionResponse{
		UID:     rqst.Request.UID,
		Allowed: true,
	}

}
func validate(w http.ResponseWriter, r *http.Request) {
	klog.V(2).Info("[serve] Entering")
	klog.V(2).Infof("[serve] Method: %s", r.Method)

	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("[serve] Content-Type=%s, expect application/json", contentType)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	klog.V(2).Info(fmt.Sprintf("[serve] Body:\n%s", body))

	rqst := v1beta1.AdmissionReview{}

	sch := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(sch)
	_ = apiextv1beta1.AddToScheme(sch)

	decode := serializer.NewCodecFactory(sch).UniversalDeserializer().Decode

	_, _, err := decode(body, nil, &rqst)
	if err != nil {
		klog.Errorf("[serve] Unable to deserialize request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if rqst.Request == nil {
		klog.Error("[serve] Admission Review request is nil")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	klog.V(2).Infof("[serve] Request:\n%+v", rqst)

	resp := validateConfiguration(rqst)
	klog.V(2).Info("[serve] Response:\n+v", resp)

	bytes, err := json.Marshal(&v1beta1.AdmissionReview{
		Response: resp,
	})
	if err != nil {
		klog.Errorf("[serve] Unable to marshal response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(bytes)
}

func main() {
	// Ensure klog flags (--logtostderr, -v) are enabled
	klog.InitFlags(nil)
	flag.Parse()

	klog.V(2).Infof("[main] Loading key-pair [%s, %s]", *crtFile, *keyFile)
	cert, err := tls.LoadX509KeyPair(*crtFile, *keyFile)
	if err != nil {
		klog.Fatal(err)
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	http.HandleFunc("/validate", validate)
	addr := fmt.Sprintf(":%d", *port)
	server := &http.Server{
		Addr:      addr,
		TLSConfig: config,
	}
	klog.V(2).Infof("[main] Starting Server [%s]", addr)
	log.Fatal(server.ListenAndServeTLS("", ""))
}
