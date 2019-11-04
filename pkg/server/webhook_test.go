package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/nsf/jsondiff" // for json diffing patches
	"github.com/tumblr/k8s-sidecar-injector/internal/pkg/config"
	_ "github.com/tumblr/k8s-sidecar-injector/internal/pkg/testing"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	sidecars     = "test/fixtures/sidecars"
	jsondiffopts = jsondiff.DefaultConsoleOptions()

	// all these configs are deserialized into metav1.ObjectMeta structs
	obj1             = "test/fixtures/k8s/object1.yaml"
	obj2             = "test/fixtures/k8s/object2.yaml"
	env1             = "test/fixtures/k8s/env1.yaml"
	obj3Missing      = "test/fixtures/k8s/object3-missing.yaml"
	obj4             = "test/fixtures/k8s/object4.yaml"
	obj5             = "test/fixtures/k8s/object5.yaml"
	obj6             = "test/fixtures/k8s/object6.yaml"
	obj7             = "test/fixtures/k8s/object7.yaml"
	obj7v2           = "test/fixtures/k8s/object7-v2.yaml"
	obj7v3           = "test/fixtures/k8s/object7-v3.yaml"
	ignoredNamespace = "test/fixtures/k8s/ignored-namespace-pod.yaml"
	badSidecar       = "test/fixtures/k8s/bad-sidecar.yaml"

	testIgnoredNamespaces = []string{"ignore-me"}

	// tests to check config loading of sidecars
	configTests = []expectedSidecarConfiguration{
		{configuration: obj1, expectedSidecar: "sidecar-test"},
		{configuration: obj2, expectedSidecar: "complex-sidecar"},
		{configuration: env1, expectedSidecar: "env1"},
		{configuration: obj3Missing, expectedSidecar: "", expectedError: ErrMissingRequestAnnotation}, // this one is missing any annotations :)
		{configuration: obj4, expectedSidecar: "", expectedError: ErrSkipAlreadyInjected},             // this one is already injected, so it should not get injected again
		{configuration: obj5, expectedSidecar: "volume-mounts"},
		{configuration: obj6, expectedSidecar: "host-aliases"},
		{configuration: obj7, expectedSidecar: "init-containers"},
		{configuration: obj7v2, expectedSidecar: "init-containers:v2"},
		{configuration: obj7v3, expectedSidecar: "init-containers:extra:data:v3"},
		{configuration: ignoredNamespace, expectedSidecar: "", expectedError: ErrSkipIgnoredNamespace},
		{configuration: badSidecar, expectedSidecar: "this-doesnt-exist", expectedError: ErrRequestedSidecarNotFound},
	}

	// tests to check the mutate() function for correct operation
	mutationTests = []mutationTest{
		{name: "missing-sidecar-config", allowed: true},
		{name: "sidecar-test-1", allowed: true},
		{name: "env-override", allowed: true},
	}
)

type expectedSidecarConfiguration struct {
	configuration   string
	expectedSidecar string
	expectedError   error
}

type mutationTest struct {
	// name is a file relative to test/fixtures/k8s/admissioncontrol/request/ ending in .yaml
	//  which is the v1beta1.AdmissionRequest object passed to mutate
	name    string
	allowed bool
}

func TestLoadConfig(t *testing.T) {
	expectedNumInjectionConfigs := 8
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

	for _, test := range configTests {
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

func TestMutation(t *testing.T) {
	c, err := config.LoadConfigDirectory(sidecars)
	if err != nil {
		t.Error(err)
		t.Fail()
	}
	c.AnnotationNamespace = "injector.unittest.com"

	s := &WebhookServer{
		Config: c,
		Server: &http.Server{
			Addr: ":6969",
		},
	}

	for _, test := range mutationTests {
		// now, try to perform the mutation on the k8s object
		var req v1beta1.AdmissionRequest
		reqFile := fmt.Sprintf("test/fixtures/k8s/admissioncontrol/request/%s.yaml", test.name)
		resPatchFile := fmt.Sprintf("test/fixtures/k8s/admissioncontrol/patch/%s.json", test.name)
		// load the AdmissionRequest object
		reqData, err := ioutil.ReadFile(reqFile)
		if err != nil {
			t.Errorf("%s: unable to load AdmissionRequest object: %v", reqFile, err)
			t.Fail()
		}
		if err := yaml.Unmarshal(reqData, &req); err != nil {
			t.Errorf("%s: unable to unmarshal AdmissionRequest yaml: %v", reqFile, err)
			t.Fail()
		}

		// stuff the request into mutate, and catch the response
		res := s.mutate(&req)

		// extract this field, so we can diff json separate from the AdmissionResponse object
		resPatch := res.Patch
		res.Patch = nil // zero this field out

		if test.allowed != res.Allowed {
			t.Errorf("expected AdmissionResponse.Allowed=%v differed from received AdmissionResponse.Allowed=%v", test.allowed, res.Allowed)
			t.Fail()
		}

		// diff the JSON patch object with expected JSON loaded from disk
		// we do this because this is way easier on the eyes than diffing
		// a yaml base64 encoded string
		if _, err := os.Stat(resPatchFile); err == nil {
			t.Logf("Loading patch data from %s...", resPatchFile)
			expectedPatchData, err := ioutil.ReadFile(resPatchFile)
			if err != nil {
				t.Error(err)
				t.Fail()
			}
			difference, diffString := jsondiff.Compare(expectedPatchData, resPatch, &jsondiffopts)
			if difference != jsondiff.FullMatch {
				t.Errorf("received AdmissionResponse.patch field differed from expected with %s (%s) (actual on left, expected on right):\n%s", resPatchFile, difference.String(), diffString)
			}

		}

	}
}
