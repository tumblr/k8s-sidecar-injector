package server

import (
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/tumblr/k8s-sidecar-injector/internal/pkg/config"
	_ "github.com/tumblr/k8s-sidecar-injector/internal/pkg/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	sidecars = "test/fixtures/sidecars"

	// all these configs are deserialized into metav1.ObjectMeta structs
	obj1             = "test/fixtures/k8s/object1.yaml"
	obj2             = "test/fixtures/k8s/object2.yaml"
	env1             = "test/fixtures/k8s/env1.yaml"
	obj3Missing      = "test/fixtures/k8s/object3-missing.yaml"
	obj4             = "test/fixtures/k8s/object4.yaml"
	obj5             = "test/fixtures/k8s/object5.yaml"
	obj6             = "test/fixtures/k8s/object6.yaml"
	ignoredNamespace = "test/fixtures/k8s/ignored-namespace-pod.yaml"
	badSidecar       = "test/fixtures/k8s/bad-sidecar.yaml"

	testIgnoredNamespaces = []string{"ignore-me"}
)

type expectedSidecarConfiguration struct {
	configuration   string
	expectedSidecar string
	expectedError   error
}

func TestLoadConfig(t *testing.T) {
	expectedNumInjectionConfigs := 5
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
	tests := []expectedSidecarConfiguration{
		{configuration: obj1, expectedSidecar: "sidecar-test"},
		{configuration: obj2, expectedSidecar: "complex-sidecar"},
		{configuration: env1, expectedSidecar: "env1"},
		{configuration: obj3Missing, expectedSidecar: "", expectedError: ErrMissingRequestAnnotation}, // this one is missing any annotations :)
		{configuration: obj4, expectedSidecar: "", expectedError: ErrSkipAlreadyInjected},             // this one is already injected, so it should not get injected again
		{configuration: obj5, expectedSidecar: "volume-mounts"},
		{configuration: obj6, expectedSidecar: "init-sidecar"},
		{configuration: ignoredNamespace, expectedSidecar: "", expectedError: ErrSkipIgnoredNamespace},
		{configuration: badSidecar, expectedSidecar: "this-doesnt-exist", expectedError: ErrRequestedSidecarNotFound},
	}

	for _, test := range tests {
		data, err := ioutil.ReadFile(test.configuration)
		if err != nil {
			t.Errorf("unable to load object metadata yaml: %v", err)
			t.Fail()
		}

		var obj *metav1.ObjectMeta
		if err := yaml.Unmarshal(data, &obj); err != nil {
			t.Errorf("unable to unmarshal object metadata yaml: %v", err)
			t.Fail()
		}
		key, err := s.getSidecarConfigurationRequested(testIgnoredNamespaces, obj)
		if err != test.expectedError {
			t.Errorf("%s: error %v did not match %v", test.configuration, err, test.expectedError)
			t.Fail()
		}
		if key != test.expectedSidecar {
			t.Errorf("%s: expected sidecar to be %v but was %v instead", test.configuration, test.expectedSidecar, key)
			t.Fail()
		}
	}
}
