/*
 * Copyright 2020 VMware, Inc.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"

	"log"
	"net/http"
	"os"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"

	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

var PodPath = "../../../example/pod.yaml"
var MemPodPath = "../../../example/pod-memory.yaml"
var StaticPod corev1.Pod
var StaticMemPod corev1.Pod
var CfgPath = "../../deployment/bitfusion_injector_webhook_configmap.yaml"
var Cfg corev1.ConfigMap

//var vfcfgstr = `initContainers:
//- name: vfinitname
//  image: vfinitimage
//containers:
//- name: vfcontainername
//  image: vfcontainerimage
//volumes:
//- name: vfvolumes
//  emptyDir: {}
//`

var vfcfgstr = `
initContainers:
- name: populate
  image: bitfusiondeviceplugin/bitfusion-client:test
  command: [/bin/bash, -c, "
      cp -ra /bitfusion/* /bitfusion-distro/ &&
      cp /root/.bitfusion/client.yaml /client &&
      cp -r BITFUSION_CLIENT_OPT_PATH /workload-container-opt
      "]
`

var TestSidecarConfig Config

func init() {
	if err := json.Unmarshal(conver(PodPath), &StaticPod); err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(conver(MemPodPath), &StaticMemPod); err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(conver(CfgPath), &Cfg); err != nil {
		log.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte(vfcfgstr), &TestSidecarConfig); err != nil {
		log.Fatal(err)
	}

	StaticMemPod.Spec.Containers[0].Resources.Requests = StaticMemPod.Spec.Containers[0].Resources.Limits
	StaticPod.Spec.Containers[0].Resources.Requests = StaticPod.Spec.Containers[0].Resources.Limits

}

func conver(path string) []byte {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(content), 100)
	var rawObj runtime.RawExtension
	if err = decoder.Decode(&rawObj); err != nil {
		log.Fatal(err)
	}

	return rawObj.Raw
}

func TestWebhookServer_Mutate(t *testing.T) {
	ar := v1beta1.AdmissionReview{
		Request: &v1beta1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: conver(PodPath),
			},
		},
	}
	mutatingWebhookSv := &WebhookServer{
		SidecarConfig: &TestSidecarConfig,
		Server: &http.Server{
			Addr: "8888",
			//TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},
	}
	admissionResponse := mutatingWebhookSv.mutate(&ar)
	t.Log(admissionResponse)
	ar = v1beta1.AdmissionReview{
		Request: &v1beta1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: []byte(""),
			},
		},
	}
	admissionResponse = mutatingWebhookSv.mutate(&ar)
	t.Log(admissionResponse)
}

type responseWriter struct {
}

func (r *responseWriter) Header() http.Header {
	res := http.Header{}
	res.Add("test", "testvalue")
	return res
}
func (r *responseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (r *responseWriter) WriteHeader(statusCode int) {}

func TestWebhookServer_Serve(t *testing.T) {
	mutatingWebhookSv := &WebhookServer{
		SidecarConfig: &TestSidecarConfig,
		Server: &http.Server{
			Addr: "8888",
			//TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},
	}

	f, _ := os.Open(PodPath)

	req := http.Request{}
	rw := responseWriter{}

	mutatingWebhookSv.Serve(&rw, &req)
	req = http.Request{
		Body: f,
	}
	mutatingWebhookSv.Serve(&rw, &req)

	f.Close()
	ar := v1beta1.AdmissionReview{
		Request: &v1beta1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: conver(PodPath),
			},
		},
	}
	content, _ := yaml.Marshal(ar)
	ioutil.WriteFile("ar.yaml", content, 0644)
	f, _ = os.Open("ar.yaml")

	header := http.Header{}
	header.Add("Content-Type", "application/json")
	req = http.Request{
		Body:   f,
		Header: header,
	}
	t.Log(req.Header.Get("Content-Type") == "application/json")

	mutatingWebhookSv.Serve(&rw, &req)
	f.Close()
}

func TestUpdateBFResource(t *testing.T) {
	testPod := StaticPod.DeepCopy()

	var verifyList []int64
	var emptyQuantity resource.Quantity

	for _, container := range testPod.Spec.Containers {
		gpuNum := container.Resources.Requests[bitFusionGPUResourceNum]
		if gpuNum == emptyQuantity {
			continue
		}
		gpuPartial := container.Resources.Requests[bitFusionGPUResourcePartial]
		if gpuPartial == emptyQuantity {
			gpuPartial.Set(100)
		}

		verifyList = append(verifyList, gpuNum.Value()*gpuPartial.Value())
	}
	bfClientConfig := BFClientConfig{"/bitfusion/bitfusion-client-centos7-2.5.0-10/usr/bin/bitfusion",
		"/bitfusion/bitfusion-client-centos7-2.5.0-10/opt/bitfusion/2.5.0-fd3e4839/x86_64-linux-gnu/lib/:$LD_LIBRARY_PATH"}
	patchs, err := updateBFResource(testPod.Spec.Containers, "spec/containers", bfClientConfig)
	if err != nil {
		t.Fatal(err)
	}

	for _, patch := range patchs {
		t.Log("Op: ", patch.Op)
		t.Log("Path ", patch.Path)
		t.Log("Value: ", patch.Value)
	}

	for _, container := range testPod.Spec.Containers {
		gpuResource := container.Resources.Requests[bitFusionGPUResource]
		if gpuResource != emptyQuantity {
			//assert.Equal(t, gpuResource.Value(), verifyList[0])
		}

		//verifyList = verifyList[1:]
	}
	p := testPod.Spec.Containers[0].Resources.Requests[bitFusionGPUResourcePartial]
	p.Set(101)
	testPod.Spec.Containers[0].Resources.Requests[bitFusionGPUResourcePartial] = p
	_, err = updateBFResource(testPod.Spec.Containers, "spec/containers", bfClientConfig)
	t.Log(err)

}

func TestConstructBitfusionDistroMap(t *testing.T) {
	type args struct {
		configFile string
	}

	tests := []struct {
		name    string
		args    args
		want    *map[interface{}]interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{name: "1", args: args{configFile: "./bitfusion-client-configmap.yaml"},
			want: nil, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConstructBitfusionDistroInfo(tt.args.configFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConstructBitfusionDistroInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConstructBitfusionDistroInfo() got = %v, want %v", got, tt.want)
			}
		})
	}
}
