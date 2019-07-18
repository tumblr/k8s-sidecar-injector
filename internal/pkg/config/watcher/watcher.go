// Package watcher is a module that handles talking to the k8s api, and watching ConfigMaps for a set of configurations, and emitting them when
// they change.
package watcher

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/golang/glog"
	"github.com/tumblr/k8s-sidecar-injector/internal/pkg/config"
	"k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	k8sv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	serviceAccountNamespaceFilePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

// WatchChannelClosedError: should restart watcher
var WatchChannelClosedError = errors.New("watcher channel has closed")

// K8sConfigMapWatcher is a struct that connects to the API and collects, parses, and emits sidecar configurations
type K8sConfigMapWatcher struct {
	Config
	client k8sv1.CoreV1Interface
}

// New creates a new K8sConfigMapWatcher
func New(cfg Config) (*K8sConfigMapWatcher, error) {
	c := K8sConfigMapWatcher{Config: cfg}
	if c.Namespace == "" {
		// ENHANCEMENT: support downward API/env vars instead? https://github.com/kubernetes/kubernetes/blob/release-1.0/docs/user-guide/downward-api.md
		// load from file on disk for serviceaccount: /var/run/secrets/kubernetes.io/serviceaccount/namespace
		ns, err := ioutil.ReadFile(serviceAccountNamespaceFilePath)
		if err != nil {
			return nil, fmt.Errorf("%s: maybe you should specify --configmap-namespace if you are running outside of kubernetes", err.Error())
		}
		if string(ns) != "" {
			c.Namespace = string(ns)
			glog.V(2).Infof("Inferred ConfigMap search namespace=%s from %s", c.Namespace, serviceAccountNamespaceFilePath)
		}
	}
	var (
		err       error
		k8sConfig *rest.Config
	)
	if c.Kubeconfig != "" || c.MasterURL != "" {
		glog.V(2).Infof("Creating Kubernetes client from kubeconfig=%s with masterurl=%s", c.Kubeconfig, c.MasterURL)
		k8sConfig, err = clientcmd.BuildConfigFromFlags(c.MasterURL, c.Kubeconfig)
		if err != nil {
			return nil, err
		}
	} else {
		glog.V(2).Infof("Creating Kubernetes client from in-cluster discovery")
		k8sConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}
	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, err
	}

	c.client = clientset.CoreV1()
	err = validate(&c)
	if err != nil {
		return nil, fmt.Errorf("validation failed for K8sConfigMapWatcher: %s", err.Error())
	}
	glog.V(2).Infof("Created ConfigMap watcher: apiserver=%s namespace=%s watchlabels=%v", k8sConfig.Host, c.Namespace, c.ConfigMapLabels)
	return &c, nil
}

func validate(c *K8sConfigMapWatcher) error {
	if c == nil {
		return fmt.Errorf("configmap watcher was nil")
	}
	if c.Namespace == "" {
		return fmt.Errorf("namespace is empty")
	}
	if c.ConfigMapLabels == nil {
		return fmt.Errorf("configmap labels was an uninitialized map")
	}
	if c.client == nil {
		return fmt.Errorf("k8s client was not setup properly")
	}
	return nil
}

// Watch watches for events impacting watched ConfigMaps and emits their events across a channel
func (c *K8sConfigMapWatcher) Watch(ctx context.Context, notifyMe chan<- interface{}) error {
	glog.V(3).Infof("Watching for ConfigMaps for changes on namespace=%s with labels=%v", c.Namespace, c.ConfigMapLabels)
	watcher, err := c.client.ConfigMaps(c.Namespace).Watch(metav1.ListOptions{
		LabelSelector: mapStringStringToLabelSelector(c.ConfigMapLabels),
	})
	if err != nil {
		return fmt.Errorf("unable to create watcher (possible serviceaccount RBAC/ACL failure?): %s", err.Error())
	}
	defer watcher.Stop()
	for {
		select {
		case e, ok := <-watcher.ResultChan():
			// channel may closed caused by HTTP timeout, should restart watcher
			// detail at https://github.com/kubernetes/client-go/issues/334
			if !ok {
				glog.Errorf("channel has closed, should restart watcher")
				return WatchChannelClosedError
			}
			if e.Type == watch.Error {
				return apierrs.FromObject(e.Object)
			}
			glog.V(3).Infof("event: %s %s", e.Type, e.Object.GetObjectKind())
			switch e.Type {
			case watch.Added:
				fallthrough
			case watch.Modified:
				fallthrough
			case watch.Deleted:
				// signal reconciliation of all InjectionConfigs
				glog.V(3).Infof("signalling event received from watch channel: %s %s", e.Type, e.Object.GetObjectKind())
				notifyMe <- struct{}{}
			default:
				glog.Errorf("got unsupported event %s for %s! skipping", e.Type, e.Object.GetObjectKind())
			}
			// events! yay!
		case <-ctx.Done():
			glog.V(2).Infof("stopping configmap watcher, context indicated we are done")
			// clean up, we cancelled the context, so stop the watch
			return nil
		}
	}
}

func mapStringStringToLabelSelector(m map[string]string) string {
	// https://github.com/kubernetes/apimachinery/issues/47
	return labels.Set(m).String()
}

// Get fetches all matching ConfigMaps
func (c *K8sConfigMapWatcher) Get() (cfgs []*config.InjectionConfig, err error) {
	glog.V(1).Infof("Fetching ConfigMaps...")
	clist, err := c.client.ConfigMaps(c.Namespace).List(metav1.ListOptions{
		LabelSelector: mapStringStringToLabelSelector(c.ConfigMapLabels),
	})
	if err != nil {
		return cfgs, err
	}
	glog.V(1).Infof("Fetched %d ConfigMaps", len(clist.Items))
	for _, cm := range clist.Items {
		injectionConfigsForCM, err := InjectionConfigsFromConfigMap(cm)
		if err != nil {
			return cfgs, fmt.Errorf("error getting ConfigMaps from API: %s", err.Error())
		}
		glog.V(1).Infof("Found %d InjectionConfigs in %s", len(injectionConfigsForCM), cm.ObjectMeta.Name)
		cfgs = append(cfgs, injectionConfigsForCM...)
	}
	return cfgs, nil
}

// InjectionConfigsFromConfigMap parse items in a configmap into a list of InjectionConfigs
func InjectionConfigsFromConfigMap(cm v1.ConfigMap) ([]*config.InjectionConfig, error) {
	ics := []*config.InjectionConfig{}
	for name, payload := range cm.Data {
		glog.V(3).Infof("Parsing %s/%s:%s into InjectionConfig", cm.ObjectMeta.Namespace, cm.ObjectMeta.Name, name)
		ic, err := config.LoadInjectionConfig(strings.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("error parsing ConfigMap %s item %s into injection config: %s", cm.ObjectMeta.Name, name, err.Error())
		}
		glog.V(2).Infof("Loaded InjectionConfig %s from ConfigMap %s:%s", ic.Name, cm.ObjectMeta.Name, name)
		ics = append(ics, ic)
	}
	return ics, nil
}
