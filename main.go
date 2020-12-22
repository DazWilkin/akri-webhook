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

	v1 "k8s.io/api/admission/v1"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog"

	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	crtFile = flag.String("tls-crt-file", "", "TLS certificate file")
	keyFile = flag.String("tls-key-file", "", "TLS key file")
	port    = flag.Int("port", 0, "Webhook Port")
)

func toAdmissionResponse(err error) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

type admitFunc func(v1beta1.AdmissionReview) *v1beta1.AdmissionResponse

func serve(w http.ResponseWriter, r *http.Request) {
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

	klog.V(2).Info(fmt.Sprintf("[serve] Handling request:\n%s", body))

	rqst := v1beta1.AdmissionReview{}

	sch := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(sch)
	_ = apiextv1beta1.AddToScheme(sch)

	decode := serializer.NewCodecFactory(sch).UniversalDeserializer().Decode

	obj, gvk, err := decode(body, nil, &rqst)
	if err != nil {
		klog.Errorf("Unable to deserialize request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	klog.V(2).Infof("[serve] Request:\n%+v", rqst)
	klog.V(2).Infof("[serve] Runtime Object:\n%+v", obj)
	klog.V(2).Infof("[serve] Schema GroupVersionKind:\n%+v", gvk)

	if rqst.Request == nil {
		klog.Error("Admission Review request is nil")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Valid Configuration?
	allowed := true

	klog.V(2).Info("[serve] Constructing response")
	resp := v1beta1.AdmissionReview{
		Response: &v1beta1.AdmissionResponse{
			UID:     rqst.Request.UID,
			Allowed: allowed,
		},
	}
	bytes, err := json.Marshal(&resp)
	if err != nil {
		klog.Errorf("Unable to marshal response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(bytes)
}

func admitCustomResource(rqst v1.AdmissionReview) *v1beta1.AdmissionResponse {
	klog.V(2).Info("[admitCustomResource] Entering")
	cr := struct {
		metav1.ObjectMeta
		Data map[string]string
	}{}

	raw := rqst.Request.Object.Raw
	err := json.Unmarshal(raw, &cr)
	if err != nil {
		klog.Error(err)
		return toAdmissionResponse(err)
	}

	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = true
	for k, v := range cr.Data {
		if k == "webhook-e2e-test" && v == "webhook-disallow" {
			reviewResponse.Allowed = false
			reviewResponse.Result = &metav1.Status{
				Reason: "the custom resource contains unwanted data",
			}
		}
	}
	return &reviewResponse
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

	http.HandleFunc("/admit-custom-resource", serve)
	addr := fmt.Sprintf(":%d", *port)
	server := &http.Server{
		Addr:      addr,
		TLSConfig: config,
	}
	klog.V(2).Infof("[main] Starting Server [%s]", addr)
	log.Fatal(server.ListenAndServeTLS("", ""))
}
