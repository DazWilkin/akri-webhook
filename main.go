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
	"net/http"

	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
)

var (
	crtFile = flag.String("tls-crt-file", "", "TLS certificate file")
	keyFile = flag.String("tls-key-file", "", "TLS key file")
	port    = flag.Int("port", 0, "Webhook Port")
)

func check(v interface{}, deserialized interface{}) error {
	// klog.V(2).Infof("check:\nv:\n%+v\nw:\n%+v", v, deserialized)
	if deserialized == nil {
		return fmt.Errorf("Input (%v) is not consistent with expected value", v)
	}
	switch v := v.(type) {
	case []interface{}:
		for i, v := range v {
			err := check(v, deserialized.([]interface{})[i])
			if err != nil {
				return fmt.Errorf("Input index (%v) is not parsed correctly: %v", i, err)
			}
		}
	case map[string]interface{}:
		for k, v := range v {
			err := check(v, deserialized.(map[string]interface{})[k])
			if err != nil {
				return fmt.Errorf("Input index (%v) is not parsed correctly: %v", k, err)
			}
		}
	case map[interface{}]interface{}:
		for k, v := range v {
			err := check(v, deserialized.(map[interface{}]interface{})[k])
			if err != nil {
				return fmt.Errorf("Input key (%v) is not parsed correctly: %v", k, err)
			}
		}
	default:
		if v != deserialized {
			return fmt.Errorf("Input (%v) is not consistent with parsed (%v)", v, deserialized)
		}
		return nil
	}

	return nil
}

func validateConfiguration(rqst *v1.AdmissionRequest) *v1.AdmissionResponse {
	resp := &v1.AdmissionResponse{
		UID:     rqst.UID,
		Allowed: false,
		Result:  &metav1.Status{},
	}

	// See: https://github.com/kubernetes/apimachinery/issues/102
	raw := rqst.Object.Raw

	if len(raw) == 0 {
		resp.Result.Message = "AdmissionReview Request Object contains no data"
		return resp
	}

	// Untyped
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		resp.Result.Message = err.Error()
		return resp
	}

	// Typed (Configuration)
	var c Configuration
	if err := json.Unmarshal(raw, &c); err != nil {
		resp.Result.Message = err.Error()
		return resp
	}

	reserialized, err := json.Marshal(c)
	if err != nil {
		resp.Result.Message = err.Error()
		return resp
	}

	var deserialized interface{}
	if err := json.Unmarshal(reserialized, &deserialized); err != nil {
		resp.Result.Message = err.Error()
		return resp
	}

	if err := check(v, deserialized); err != nil {
		resp.Result.Message = err.Error()
		return resp
	}

	// Otherwise, we're good!
	resp.Allowed = true
	return resp

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

	rqst := v1.AdmissionReview{}

	sch := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(sch)

	decode := serializer.NewCodecFactory(sch).UniversalDeserializer().Decode

	_, _, err := decode(body, nil, &rqst)
	if err != nil {
		klog.Errorf("[serve] Unable to deserialize request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	klog.V(2).Infof("[serve] Request:\n%+v", rqst)

	if rqst.Request == nil {
		klog.Error("[serve] Admission Review request is nil")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	resp := validateConfiguration(rqst.Request)

	klog.V(2).Infof("[serve] Response:\n%+v", resp)

	bytes, err := json.Marshal(&v1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: resp,
	})
	if err != nil {
		klog.Errorf("Unable to marshal response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	klog.V(2).Infof("[serve] Response:\n%+v", string(bytes))
	w.Write(bytes)
}

func main() {
	// Ensure klog flags (--logtostderr, --v) are enabled
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
	klog.Fatal(server.ListenAndServeTLS("", ""))
}
