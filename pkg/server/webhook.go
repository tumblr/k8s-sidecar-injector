package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tumblr/k8s-sidecar-injector/pkg/config"
	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()

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
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10},
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
	_ = admissionregistrationv1beta1.AddToScheme(runtimeScheme)
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

// Check whether the target resoured need to be mutated
func (whsvr *WebhookServer) requiredMutation(ignoredList []string, metadata *metav1.ObjectMeta) string {
	// skip special kubernete system namespaces
	for _, namespace := range ignoredList {
		if metadata.Namespace == namespace {
			glog.Infof("Skip mutation for %v in ignorednamespace: %v", metadata.Name, metadata.Namespace)
			return ""
		}
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	status := annotations[whsvr.Config.AnnotationNamespace+"/status"]

	// determine whether to perform mutation based on annotation for the target resource
	requestedInjection := ""
	if strings.ToLower(status) != "injected" {
		requestedInjection = strings.ToLower(annotations[whsvr.Config.AnnotationNamespace+"/request"])
		if !whsvr.Config.HasInjectionConfig(requestedInjection) {
			glog.Errorf("Mutation policy for %v/%v: status:%q requested injection: %s was not in configuration, skipping", metadata.Namespace, metadata.Name, status, requestedInjection)
			return ""
		}
	}

	glog.Infof("Mutation policy for %v/%v: status:%q injection:%s", metadata.Namespace, metadata.Name, status, requestedInjection)
	return requestedInjection
}

func setEnvironment(target []corev1.Container, addedEnv []corev1.EnvVar) (patch []patchOperation) {
	var value interface{}
	for containerIndex, container := range target {
		// for each container in the spec, determine if we want to patch with any env vars
		first := len(container.Env) == 0
		for _, add := range addedEnv {
			path := fmt.Sprintf("/spec/containers/%d/env", containerIndex)
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

func addVolumes(target, added []corev1.Volume, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.Volume{add}
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

// for containers, add any env vars that are not already defined in the Env list.
// this does _not_ return patches; this is intended to be used only on containers defined
// in the injection config, so the resources do not exist yet in the k8s api (thus no patch needed)
func mergeEnvVars(envs []corev1.EnvVar, containers []corev1.Container) []corev1.Container {
	mutatedContainers := []corev1.Container{}
	for _, c := range containers {
		for _, newEnv := range envs {
			// check each container for each env var by name.
			// if the container has a matching name, dont override!
			skip := false
			for _, origEnv := range c.Env {
				if origEnv.Name == newEnv.Name {
					skip = true
					break
				}
			}
			if !skip {
				c.Env = append(c.Env, newEnv)
			}
		}
		mutatedContainers = append(mutatedContainers, c)
	}
	return mutatedContainers
}

func updateAnnotations(target map[string]string, added map[string]string) (patch []patchOperation) {
	for key, value := range added {
		if target == nil || target[key] == "" {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					key: value,
				},
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/metadata/annotations/" + key,
				Value: value,
			})
		}
	}
	return patch
}

// create mutation patch for resoures
func createPatch(pod *corev1.Pod, inj *config.InjectionConfig, annotations map[string]string) ([]byte, error) {
	var patch []patchOperation

	// first, make sure any injected containers in our config get the EnvVars injected
	// this mutates inj.Containers with our environment vars
	mutatedInjectedContainers := mergeEnvVars(inj.Environment, inj.Containers)
	// next, patch containers with our injected containers
	patch = append(patch, addContainers(pod.Spec.Containers, mutatedInjectedContainers, "/spec/containers")...)
	// now, patch all existing containers with the env vars
	patch = append(patch, setEnvironment(pod.Spec.Containers, inj.Environment)...)
	// now, add volumes and set annotations
	patch = append(patch, addVolumes(pod.Spec.Volumes, inj.Volumes, "/spec/volumes")...)
	patch = append(patch, updateAnnotations(pod.Annotations, annotations)...)

	return json.Marshal(patch)
}

// main mutation process
func (whsvr *WebhookServer) mutate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := ar.Request
	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		glog.Errorf("Could not unmarshal raw object: %v", err)
		injectionCounter.With(prometheus.Labels{"status": "error", "reason": "unmarshal_error"}).Inc()
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	glog.Infof("AdmissionReview for Kind=%v, Namespace=%v Name=%v (%v) UID=%v patchOperation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, pod.Name, req.UID, req.Operation, req.UserInfo)

	// determine whether to perform mutation
	injectionKey := whsvr.requiredMutation(ignoredNamespaces, &pod.ObjectMeta)
	if injectionKey == "" {
		glog.Infof("Skipping mutation for %s/%s because no injection request in annotations", pod.Namespace, pod.Name)
		injectionCounter.With(prometheus.Labels{"status": "skipped", "reason": "no_annotation", "requested": injectionKey}).Inc()
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	injectionConfig, err := whsvr.Config.GetInjectionConfig(injectionKey)
	if err != nil {
		glog.Errorf("Error getting injection config for %s, so we will fail open: %s", injectionConfig, err.Error())
		// dont prevent pods from launching! just return allowed
		injectionCounter.With(prometheus.Labels{"status": "skipped", "reason": "missing_config", "requested": injectionKey}).Inc()
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	// Workaround: https://github.com/kubernetes/kubernetes/issues/57982
	applyDefaultsWorkaround(injectionConfig.Containers, injectionConfig.Volumes)
	annotations := map[string]string{config.InjectionStatusAnnotation: "injected"}
	patchBytes, err := createPatch(&pod, injectionConfig, annotations)
	if err != nil {
		injectionCounter.With(prometheus.Labels{"status": "error", "reason": "patching_error", "requested": injectionKey}).Inc()
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	glog.Infof("AdmissionResponse: patch=%v\n", string(patchBytes))
	injectionCounter.With(prometheus.Labels{"status": "success", "reason": "all_groovy", "requested": injectionKey}).Inc()
	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
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
