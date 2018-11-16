package server

import (
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/tumblr/k8s-sidecar-injector/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	sidecars    = "../../test/fixtures/sidecars"
	obj1        = "../../test/fixtures/k8s/object1.yaml"
	obj2        = "../../test/fixtures/k8s/object2.yaml"
	env1        = "../../test/fixtures/k8s/env1.yaml"
	obj3Missing = "../../test/fixtures/k8s/object3-missing.yaml"
	obj4        = "../../test/fixtures/k8s/object4.yaml"
)

func TestLoadConfig(t *testing.T) {
	expectedNumInjectionConfigs := 3
	c, err := config.LoadConfigDirectory(sidecars)
	if err != nil {
		t.Error(err)
		t.Fail()
	}
	c.AnnotationNamespace = "injector.unittest.com"
	if len(c.Injections) != expectedNumInjectionConfigs {
		t.Errorf("expected %d injection configs to be loaded from %s, but got %d", expectedNumInjectionConfigs, sidecars, len(c.Injections))
		t.Fail()
	}
	if c.AnnotationNamespace != "injector.unittest.com" {
		t.Errorf("expected injector.unittest.com default AnnotationNamespace but got %s", c.AnnotationNamespace)
		t.Fail()
	}

	s := &WebhookServer{
		Config: c,
		Server: &http.Server{
			Addr: ":6969",
		},
	}

	// load some objects that are k8s metadata objects
	objects := map[string]string{
		obj1:        "sidecar-test",
		obj2:        "complex-sidecar",
		env1:        "env1",
		obj3Missing: "", // this one is missing any annotations :)
		obj4:        "", // this one is already injected, so it should not get injected again
	}
	for f, k := range objects {
		data, err := ioutil.ReadFile(f)
		if err != nil {
			t.Errorf("unable to load object metadata yaml: %v", err)
			t.Fail()
		}

		var obj *metav1.ObjectMeta
		if err := yaml.Unmarshal(data, &obj); err != nil {
			t.Errorf("unable to unmarshal object metadata yaml: %v", err)
			t.Fail()
		}
		key := s.requiredMutation([]string{}, obj)
		if key != k {
			t.Errorf("%s: expected required mutation key to be %v but was %v instead", f, k, key)
			t.Fail()
		}
	}
}
