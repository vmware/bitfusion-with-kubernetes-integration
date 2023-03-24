/*
 * Copyright 2020 VMware, Inc.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package webhook

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"net/http"
)

// Bitfusion client binary path and environment variables value of LD_LIBRARY_PATH
type BFClientConfig struct {
	BinaryPath  string
	EnvVariable string
}

// BitfusionClients configuration for each Bitfusion client in different OS
type BitfusionClients struct {
	BitfusionVersion string `yaml:"BitfusionVersion"`
	OSVersion        string `yaml:"OSVersion"`
	BinaryPath       string `yaml:"BinaryPath"`
	EnvVariable      string `yaml:"EnvVariable"`
}

// BitfusionClientDistro struct
type BitfusionClientDistro struct {
	BitfusionClients []BitfusionClients `yaml:"BitfusionClients"`
}

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()

	defaulter    = runtime.ObjectDefaulter(runtimeScheme)
	zeroQuantity = resource.Quantity{}

	injectionStatus    = ""
	BitfusionClientMap *map[string]map[string]BFClientConfig
)

var ignoredNamespaces = []string{
	metav1.NamespaceSystem,
	metav1.NamespacePublic,
}

const (
	guestOS                             = "bitfusion-client/os"
	bfVersion                           = "bitfusion-client/version"
	admissionWebhookAnnotationFilterKey = "bitfusion-client/filter"
	admissionWebhookAnnotationInjectKey = "auto-management/bitfusion"
	admissionWebhookAnnotationStatusKey = "auto-management/status"
	// "~1" is used for escape (http://jsonpatch.com/)
	bitFusionGPUResource        = "bitfusion.io/gpu"
	bitFusionGPUResourceNum     = "bitfusion.io/gpu-amount"
	bitFusionGPUResourceMemory  = "bitfusion.io/gpu-memory"
	bitFusionGPUResourcePartial = "bitfusion.io/gpu-percent"
	bitFusionOnlyInjection      = "injection"
)

// WebhookServer struct
type WebhookServer struct {
	SidecarConfig *Config
	Server        *http.Server
}

// Webhook Server parameters
type WhSvrParameters struct {
	Port                  int    // webhook server port
	CertFile              string // path to the x509 certificate for https
	KeyFile               string // path to the x509 private key matching `CertFile`
	SidecarCfgFile        string // path to sidecar injector configuration file
	BitfusionClientConfig string // path to Bitfusion client configuration file
}

// Config struct
type Config struct {
	InitContainers []corev1.Container `yaml:"initContainers"`
	Containers     []corev1.Container `yaml:"containers"`
	Volumes        []corev1.Volume    `yaml:"volumes"`
}

// patchOperation Update field(s) of a resource using strategic merge patch.
type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1beta1.AddToScheme(runtimeScheme)
	_ = corev1.AddToScheme(runtimeScheme)
}

func applyDefaultsWorkaround(containers []corev1.Container, volumes []corev1.Volume) {
	defaulter.Default(&corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: containers,
			Volumes:    volumes,
		},
	})
}

// Serve method for webhook server
func (whsvr *WebhookServer) Serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		glog.Error("Empty body")
		http.Error(w, "Empty body", http.StatusBadRequest)
		return
	}

	// Verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		glog.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		glog.Errorf("Can't decode body: %v", err)
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = whsvr.mutate(&ar)
	}

	admissionReview := v1beta1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
			admissionReview.Kind = ar.Kind
			admissionReview.APIVersion = ar.APIVersion
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		glog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	glog.Infof("Ready to write reponse ...")
	if _, err := w.Write(resp); err != nil {
		glog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

// mutate is main mutation process
func (whsvr *WebhookServer) mutate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := ar.Request
	var pod corev1.Pod
	response := &v1beta1.AdmissionResponse{}

	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		glog.Errorf("Could not unmarshal raw object: %v", err)
		response.Result = &metav1.Status{Message: err.Error()}
		return response
	}

	glog.Infof("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, pod.Name, req.UID, req.Operation, req.UserInfo)

	// Determine whether to perform mutation
	if !mutationRequired(ignoredNamespaces, &pod.ObjectMeta) {
		glog.Infof("Skipping mutation for %s/%s due to policy check", pod.Namespace, pod.Name)
		response.Allowed = true
		return response
	}

	// If user did not specify the GuestOS annotation, webhook will do nothing with the container
	os := getGuestOS(&pod.ObjectMeta)
	bfVersion := getBfVersion(&pod.ObjectMeta)
	clientMap := *BitfusionClientMap
	if os == "" || bfVersion == "" {
		response.Allowed = true
		return response
	} else {
		if _, ok := clientMap[os][bfVersion]; !ok {
			glog.Errorf("Could not find Bitfusion client info, OS=%v BFVersion=%v", os, bfVersion)
			response.Result = &metav1.Status{Message: "Could not find Bitfusion client info"}
			return response
		}
	}

	applyDefaultsWorkaround(whsvr.SidecarConfig.Containers, whsvr.SidecarConfig.Volumes)
	//annotations := map[string]string{admissionWebhookAnnotationStatusKey: "injected"}
	// Adding support for the filter parameter requires obtaining the metadata content
	metadata := &pod.ObjectMeta
	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	patchBytes, err := createPatch(&pod, whsvr.SidecarConfig, annotations, clientMap[os][bfVersion])
	if err != nil {
		response.Result = &metav1.Status{Message: err.Error()}
		return response
	}

	if err = copySecret(&req.Namespace); err != nil {
		glog.Errorf("Can't copy secret: %v", err)
		response.Result = &metav1.Status{Message: err.Error()}
		return response
	}

	response.Allowed = true
	response.Patch = patchBytes
	response.PatchType = func() *v1beta1.PatchType {
		pt := v1beta1.PatchTypeJSONPatch
		return &pt
	}()

	return response
}
