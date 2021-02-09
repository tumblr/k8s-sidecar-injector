package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tumblr/k8s-sidecar-injector/internal/pkg/config"
	v1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

const (
	// StatusInjected is the annotation value for /status that indicates an injection was already performed on this pod
	StatusInjected = "injected"
)

var (
	serviceAccountTokenMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"
	runtimeScheme                = runtime.NewScheme()
	codecs                       = serializer.NewCodecFactory(runtimeScheme)
	deserializer                 = codecs.UniversalDeserializer()

	// (https://github.com/kubernetes/kubernetes/issues/57982)
	defaulter = runtime.ObjectDefaulter(runtimeScheme)

	injectionCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "injections",
			Help: "Count of mutations/injections into a resource",
		},
		[]string{"status", "reason", "requested"},
	)

	httpReqInFlightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_in_flight_requests",
		Help: "A gauge of requests currently being served by the wrapped handler.",
	})

	httpReqCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_api_requests_total",
			Help: "A counter for requests to the wrapped handler.",
		},
		[]string{"code", "method"},
	)

	// duration is partitioned by the HTTP method and handler. It uses custom
	// buckets based on the expected request duration.
	httpReqDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "A histogram of latencies for requests.",
			Buckets: []float64{.001, .01, .05, .1, .5, 1, 5},
		},
		[]string{"handler", "method"},
	)

	// responseSize has no labels, making it a zero-dimensional
	// ObserverVec.
	httpResResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "A histogram of response sizes for requests.",
			Buckets: []float64{100, 200, 500, 900, 1500, 5000},
		},
		[]string{},
	)
)

var ignoredNamespaces = []string{
	metav1.NamespaceSystem,
	metav1.NamespacePublic,
}

// WebhookServer is a server that handles mutating admission webhooks
type WebhookServer struct {
	Config *config.Config
	Server *http.Server
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1.AddToScheme(runtimeScheme)
	// defaulting with webhooks:
	// https://github.com/kubernetes/kubernetes/issues/57982
	_ = corev1.AddToScheme(runtimeScheme)

	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(injectionCounter, httpReqInFlightGauge, httpReqCounter, httpReqDuration, httpResResponseSize)
}

func instrumentHandler(name string, h http.Handler) http.Handler {
	return promhttp.InstrumentHandlerInFlight(httpReqInFlightGauge,
		promhttp.InstrumentHandlerDuration(httpReqDuration.MustCurryWith(prometheus.Labels{"handler": name}),
			promhttp.InstrumentHandlerCounter(httpReqCounter,
				promhttp.InstrumentHandlerResponseSize(httpResResponseSize, h),
			),
		),
	)
}

// (https://github.com/kubernetes/kubernetes/issues/57982)
func applyDefaultsWorkaround(containers []corev1.Container, volumes []corev1.Volume) {
	defaulter.Default(&corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: containers,
			Volumes:    volumes,
		},
	})
}

func (whsvr *WebhookServer) statusAnnotationKey() string {
	return whsvr.Config.AnnotationNamespace + "/status"
}

func (whsvr *WebhookServer) requestAnnotationKey() string {
	return whsvr.Config.AnnotationNamespace + "/request"
}

// Check whether the target resoured need to be mutated. returns the canonicalized full name of the injection config
// if found, or an error if not.
func (whsvr *WebhookServer) getSidecarConfigurationRequested(ignoredList []string, metadata *metav1.ObjectMeta) (string, error) {
	// skip special kubernetes system namespaces
	for _, namespace := range ignoredList {
		if metadata.Namespace == namespace {
			glog.Infof("Pod %s/%s should skip injection due to ignored namespace", metadata.Name, metadata.Namespace)
			return "", ErrSkipIgnoredNamespace
		}
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	statusAnnotationKey := whsvr.statusAnnotationKey()
	requestAnnotationKey := whsvr.requestAnnotationKey()

	status, ok := annotations[statusAnnotationKey]
	if ok && strings.ToLower(status) == StatusInjected {
		glog.Infof("Pod %s/%s annotation %s=%s indicates injection already satisfied, skipping", metadata.Namespace, metadata.Name, statusAnnotationKey, status)
		return "", ErrSkipAlreadyInjected
	}

	// determine whether to perform mutation based on annotation for the target resource
	requestedInjection, ok := annotations[requestAnnotationKey]
	if !ok {
		glog.Infof("Pod %s/%s annotation %s is missing, skipping injection", metadata.Namespace, metadata.Name, requestAnnotationKey)
		return "", ErrMissingRequestAnnotation
	}
	ic, err := whsvr.Config.GetInjectionConfig(requestedInjection)
	if err != nil {
		glog.Errorf("Mutation policy for pod %s/%s: %v", metadata.Namespace, metadata.Name, err)
		return "", ErrRequestedSidecarNotFound
	}

	glog.Infof("Pod %s/%s annotation %s=%s requesting sidecar config %s", metadata.Namespace, metadata.Name, requestAnnotationKey, requestedInjection, ic.FullName())
	return ic.FullName(), nil
}

func setEnvironment(target []corev1.Container, addedEnv []corev1.EnvVar, basePath string) (patch []patchOperation) {
	var value interface{}
	for containerIndex, container := range target {
		// for each container in the spec, determine if we want to patch with any env vars
		first := len(container.Env) == 0
		for _, add := range addedEnv {
			path := fmt.Sprintf("%s/%d/env", basePath, containerIndex)
			hasKey := false
			// make sure we dont override any existing env vars; we only add, dont replace
			for _, origEnv := range container.Env {
				if origEnv.Name == add.Name {
					hasKey = true
					break
				}
			}
			if !hasKey {
				// make a patch
				value = add
				if first {
					first = false
					value = []corev1.EnvVar{add}
				} else {
					path = path + "/-"
				}
				patch = append(patch, patchOperation{
					Op:    "add",
					Path:  path,
					Value: value,
				})
			}
		}
	}
	return patch
}

func addContainers(target, added []corev1.Container, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.Container{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func setHostNetwork(target bool, addedHostNetwork bool, basePath string) (patch []patchOperation) {
	if addedHostNetwork == true {
		patch = append(patch, patchOperation{
			Op:    "replace",
			Path:  basePath,
			Value: addedHostNetwork,
		})
	}
	return patch
}

func setHostPID(target bool, addedHostPID bool, basePath string) (patch []patchOperation) {
	if addedHostPID == true {
		patch = append(patch, patchOperation{
			Op:    "replace",
			Path:  basePath,
			Value: addedHostPID,
		})
	}
	return patch
}

func addVolumes(existing, added []corev1.Volume, basePath string) (patch []patchOperation) {
	hasVolume := func(existing []corev1.Volume, add corev1.Volume) bool {
		for _, v := range existing {
			// if any of the existing volumes have the same name as test.Name, skip
			// injecting it
			if v.Name == add.Name {
				return true
			}
		}
		return false
	}
	for _, add := range added {
		value := add

		if hasVolume(existing, add) {
			continue
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  basePath + "/-",
			Value: value,
		})
	}
	return patch
}

func addVolumeMounts(target []corev1.Container, addedVolumeMounts []corev1.VolumeMount, basePath string) (patch []patchOperation) {
	var value interface{}
	for containerIndex, container := range target {
		// for each container in the spec, determine if we want to patch with any volume mounts
		first := len(container.VolumeMounts) == 0
		for _, add := range addedVolumeMounts {
			path := fmt.Sprintf("%s/%d/volumeMounts", basePath, containerIndex)
			hasKey := false
			// make sure we dont override any existing volume mounts; we only add, dont replace
			for _, origVolumeMount := range container.VolumeMounts {
				if origVolumeMount.Name == add.Name {
					hasKey = true
					break
				}
			}
			if !hasKey {
				// make a patch
				value = add
				if first {
					first = false
					value = []corev1.VolumeMount{add}
				} else {
					path = path + "/-"
				}
				patch = append(patch, patchOperation{
					Op:    "add",
					Path:  path,
					Value: value,
				})
			}
		}
	}
	return patch
}

func addHostAliases(target, added []corev1.HostAlias, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.HostAlias{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func setServiceAccount(initContainers []corev1.Container, containers []corev1.Container, sa string, basePath string) (patch []patchOperation) {
	patch = append(patch, patchOperation{
		Op:    "replace",
		Path:  path.Join(basePath, "serviceAccountName"),
		Value: sa,
	})

	// if we find any pre-existing VolumeMounts that provide the default serviceaccount token, we need to snip
	// them out, so the ServiceAccountController will create the correct VolumeMount once we patch this pod
	// volumeMounts:
	//  - name: default-token-wlfz2
	//    readOnly: true
	//    mountPath: /var/run/secrets/kubernetes.io/serviceaccount
	for icIndex, ic := range initContainers {
		for vmIndex, vm := range ic.VolumeMounts {
			if vm.MountPath == serviceAccountTokenMountPath {
				patch = append(patch, patchOperation{
					Op:   "remove",
					Path: fmt.Sprintf("%s/initContainers/%d/volumeMounts/%d", basePath, icIndex, vmIndex),
				})
			}
		}
	}
	for cIndex, c := range containers {
		for vmIndex, vm := range c.VolumeMounts {
			if vm.MountPath == serviceAccountTokenMountPath {
				patch = append(patch, patchOperation{
					Op:   "remove",
					Path: fmt.Sprintf("%s/containers/%d/volumeMounts/%d", basePath, cIndex, vmIndex),
				})
			}
		}
	}
	return patch
}

// for containers, add any env vars that are not already defined in the Env list.
// this does _not_ return patches; this is intended to be used only on containers defined
// in the injection config, so the resources do not exist yet in the k8s api (thus no patch needed)
func mergeEnvVars(envs []corev1.EnvVar, containers []corev1.Container) []corev1.Container {
	hasEnvVar := func(existing []corev1.EnvVar, add corev1.EnvVar) bool {
		for _, v := range existing {
			// if any of the existing volumes have the same name as test.Name, skip
			// injecting it
			if v.Name == add.Name {
				return true
			}
		}
		return false
	}
	mutatedContainers := []corev1.Container{}
	for _, c := range containers {
		for _, newEnv := range envs {
			// check each container for each env var by name.
			// if the container has a matching name, dont override!
			if hasEnvVar(c.Env, newEnv) {
				continue
			}
			c.Env = append(c.Env, newEnv)
		}
		mutatedContainers = append(mutatedContainers, c)
	}
	return mutatedContainers
}

func mergeVolumeMounts(volumeMounts []corev1.VolumeMount, containers []corev1.Container) []corev1.Container {
	mutatedContainers := []corev1.Container{}
	for _, c := range containers {
		for _, newVolumeMount := range volumeMounts {
			// check each container for each volume mount by name.
			// if the container has a matching name, dont override!
			skip := false
			for _, origVolumeMount := range c.VolumeMounts {
				if origVolumeMount.Name == newVolumeMount.Name {
					skip = true
					break
				}
			}
			if !skip {
				c.VolumeMounts = append(c.VolumeMounts, newVolumeMount)
			}
		}
		mutatedContainers = append(mutatedContainers, c)
	}
	return mutatedContainers
}

func updateAnnotations(target map[string]string, added map[string]string) (patch []patchOperation) {
	for key, value := range added {
		keyEscaped := strings.Replace(key, "/", "~1", -1)

		if target == nil || target[key] == "" {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:    "add",
				Path:  path.Join("/metadata/annotations", keyEscaped),
				Value: value,
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  path.Join("/metadata/annotations", keyEscaped),
				Value: value,
			})
		}
	}
	return patch
}

// create mutation patch for resoures
func createPatch(pod *corev1.Pod, inj *config.InjectionConfig, annotations map[string]string) ([]byte, error) {
	var patch []patchOperation

	// be sure to inject the serviceAccountName before adding any volumeMounts, because we must prune out any existing
	// volumeMounts that were added to support the default service account. Because this removal is by index, we splice
	// them out before appending new volumes at the end.
	if inj.ServiceAccountName != "" && (pod.Spec.ServiceAccountName == "" || pod.Spec.ServiceAccountName == "default") {
		// only override the serviceaccount name if not set in the pod spec
		patch = append(patch, setServiceAccount(pod.Spec.InitContainers, pod.Spec.Containers, inj.ServiceAccountName, "/spec")...)
	}

	{ // initcontainer injections
		// patch all existing InitContainers with the VolumeMounts+EnvVars, and add injected initcontainers
		patch = append(patch, setEnvironment(pod.Spec.InitContainers, inj.Environment, "/spec/initContainers")...)
		patch = append(patch, addVolumeMounts(pod.Spec.InitContainers, inj.VolumeMounts, "/spec/initContainers")...)
		// next, make sure any injected init containers in our config get the EnvVars and VolumeMounts injected
		// this mutates inj.InitContainers with our environment vars
		mutatedInjectedInitContainers := mergeEnvVars(inj.Environment, inj.InitContainers)
		mutatedInjectedInitContainers = mergeVolumeMounts(inj.VolumeMounts, mutatedInjectedInitContainers)
		patch = append(patch, addContainers(pod.Spec.InitContainers, mutatedInjectedInitContainers, "/spec/initContainers")...)
	}

	{ // container injections
		// now, patch all existing containers with the env vars and volume mounts, and add injected containers
		patch = append(patch, setEnvironment(pod.Spec.Containers, inj.Environment, "/spec/containers")...)
		patch = append(patch, addVolumeMounts(pod.Spec.Containers, inj.VolumeMounts, "/spec/containers")...)
		// first, make sure any injected containers in our config get the EnvVars and VolumeMounts injected
		// this mutates inj.Containers with our environment vars
		mutatedInjectedContainers := mergeEnvVars(inj.Environment, inj.Containers)
		mutatedInjectedContainers = mergeVolumeMounts(inj.VolumeMounts, mutatedInjectedContainers)
		patch = append(patch, addContainers(pod.Spec.Containers, mutatedInjectedContainers, "/spec/containers")...)
	}

	{ // pod level mutations
		// now, add hostAliases and volumes
		patch = append(patch, addHostAliases(pod.Spec.HostAliases, inj.HostAliases, "/spec/hostAliases")...)
		patch = append(patch, addVolumes(pod.Spec.Volumes, inj.Volumes, "/spec/volumes")...)
	}

	{ // now, set hostNetwork,hostPID
		patch = append(patch, setHostNetwork(pod.Spec.HostNetwork, inj.HostNetwork, "/spec/hostNetwork")...)
		patch = append(patch, setHostPID(pod.Spec.HostPID, inj.HostPID, "/spec/hostPID")...)
	}

	// last but not least, set annotations
	patch = append(patch, updateAnnotations(pod.Annotations, annotations)...)
	return json.Marshal(patch)
}

// main mutation process
func (whsvr *WebhookServer) mutate(req *v1.AdmissionRequest) *v1.AdmissionResponse {
	var pod corev1.Pod

	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		glog.Errorf("Could not unmarshal raw object: %v", err)
		injectionCounter.With(prometheus.Labels{"status": "error", "reason": "unmarshal_error", "requested": ""}).Inc()
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	glog.Infof("AdmissionReview for Kind=%s, Namespace=%s Name=%s (%s) UID=%s patchOperation=%s UserInfo=%s",
		req.Kind, req.Namespace, req.Name, pod.Name, req.UID, req.Operation, req.UserInfo)

	// determine whether to perform mutation
	injectionKey, err := whsvr.getSidecarConfigurationRequested(ignoredNamespaces, &pod.ObjectMeta)
	if err != nil {
		glog.Infof("Skipping mutation of %s/%s: %v", pod.Namespace, pod.Name, err)
		reason := GetErrorReason(err)
		injectionCounter.With(prometheus.Labels{"status": "skipped", "reason": reason, "requested": injectionKey}).Inc()
		return &v1.AdmissionResponse{
			Allowed: true,
		}
	}

	injectionConfig, err := whsvr.Config.GetInjectionConfig(injectionKey)
	if err != nil {
		glog.Errorf("Error getting injection config %s, permitting launch of pod with no sidecar injected: %s", injectionConfig, err.Error())
		// dont prevent pods from launching! just return allowed
		injectionCounter.With(prometheus.Labels{"status": "skipped", "reason": "missing_config", "requested": injectionKey}).Inc()
		return &v1.AdmissionResponse{
			Allowed: true,
		}
	}

	// Workaround: https://github.com/kubernetes/kubernetes/issues/57982
	applyDefaultsWorkaround(injectionConfig.Containers, injectionConfig.Volumes)
	annotations := map[string]string{}
	annotations[whsvr.statusAnnotationKey()] = StatusInjected
	patchBytes, err := createPatch(&pod, injectionConfig, annotations)
	if err != nil {
		injectionCounter.With(prometheus.Labels{"status": "error", "reason": "patching_error", "requested": injectionKey}).Inc()
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	glog.Infof("AdmissionResponse: patch=%v\n", string(patchBytes))
	injectionCounter.With(prometheus.Labels{"status": "success", "reason": "all_groovy", "requested": injectionKey}).Inc()
	return &v1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1.PatchType {
			pt := v1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

// MetricsHandler method for webhook server
func (whsvr *WebhookServer) MetricsHandler() http.Handler {
	return instrumentHandler("metrics", promhttp.Handler())
}

// HealthHandler returns ok
func (whsvr *WebhookServer) HealthHandler() http.Handler {
	return instrumentHandler("health", http.HandlerFunc(whsvr.healthHandler))
}

// MutateHandler method for webhook server
func (whsvr *WebhookServer) MutateHandler() http.Handler {
	return instrumentHandler("mutate", http.HandlerFunc(whsvr.mutateHandler))
}

func (whsvr *WebhookServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "d|-_-|b 🦄")
}

func (whsvr *WebhookServer) mutateHandler(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		glog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		glog.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1.AdmissionResponse
	admissionReviewRequest := v1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &admissionReviewRequest); err != nil {
		glog.Errorf("Can't decode body: %v", err)
		admissionResponse = &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		admissionResponse = whsvr.mutate(admissionReviewRequest.Request)
	}

	admissionReviewResponse := v1.AdmissionReview{}
	admissionReviewResponse.APIVersion = admissionReviewRequest.APIVersion
	admissionReviewResponse.Kind = admissionReviewRequest.Kind
	if admissionResponse != nil {
		admissionReviewResponse.Response = admissionResponse
		if admissionReviewRequest.Request != nil {
			admissionReviewResponse.Response.UID = admissionReviewRequest.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReviewResponse)
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
