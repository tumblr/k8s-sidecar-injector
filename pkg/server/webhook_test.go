package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/nsf/jsondiff" // for json diffing patches
	"github.com/tumblr/k8s-sidecar-injector/internal/pkg/config"
	_ "github.com/tumblr/k8s-sidecar-injector/internal/pkg/testing"
	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	sidecars     = "test/fixtures/sidecars"
	jsondiffopts = jsondiff.DefaultConsoleOptions()

	// all these configs are deserialized into metav1.ObjectMeta structs
	obj1             = "test/fixtures/k8s/object1.yaml"
	obj2latest       = "test/fixtures/k8s/object2latest.yaml"
	obj2v            = "test/fixtures/k8s/object2v.yaml"
	env1             = "test/fixtures/k8s/env1.yaml"
	obj3Missing      = "test/fixtures/k8s/object3-missing.yaml"
	obj4             = "test/fixtures/k8s/object4.yaml"
	obj5             = "test/fixtures/k8s/object5.yaml"
	obj6             = "test/fixtures/k8s/object6.yaml"
	obj7             = "test/fixtures/k8s/object7.yaml"
	obj7v2           = "test/fixtures/k8s/object7-v2.yaml"
	obj7v3           = "test/fixtures/k8s/object7-badrequestformat.yaml"
	ignoredNamespace = "test/fixtures/k8s/ignored-namespace-pod.yaml"
	badSidecar       = "test/fixtures/k8s/bad-sidecar.yaml"

	testIgnoredNamespaces = []string{"ignore-me"}

	// tests to check config loading of sidecars
	configTests = []expectedSidecarConfiguration{
		{configuration: obj1, expectedSidecar: "sidecar-test:latest"},
		{configuration: obj2latest, expectedSidecar: "", expectedError: ErrRequestedSidecarNotFound},
		{configuration: obj2v, expectedSidecar: "complex-sidecar:v420.69"},
		{configuration: env1, expectedSidecar: "env1:latest"},
		{configuration: obj3Missing, expectedSidecar: "", expectedError: ErrMissingRequestAnnotation}, // this one is missing any annotations :)
		{configuration: obj4, expectedSidecar: "", expectedError: ErrSkipAlreadyInjected},             // this one is already injected, so it should not get injected again
		{configuration: obj5, expectedSidecar: "volume-mounts:latest"},
		{configuration: obj6, expectedSidecar: "host-aliases:latest"},
		{configuration: obj7, expectedSidecar: "init-containers:latest"},
		{configuration: obj7v2, expectedSidecar: "init-containers:v2"},
		{configuration: obj7v3, expectedSidecar: "", expectedError: ErrRequestedSidecarNotFound},
		{configuration: ignoredNamespace, expectedSidecar: "", expectedError: ErrSkipIgnoredNamespace},
		{configuration: badSidecar, expectedSidecar: "", expectedError: ErrRequestedSidecarNotFound},
	}

	// tests to check the mutate() function for correct operation
	mutationTests = []mutationTest{
		{name: "missing-sidecar-config", allowed: true, patchExpected: false},
		{name: "sidecar-test-1", allowed: true, patchExpected: true},
		{name: "env-override", allowed: true, patchExpected: true},
		{name: "service-account", allowed: true, patchExpected: true},
		{name: "service-account-already-set", allowed: true, patchExpected: true},
		{name: "service-account-set-default", allowed: true, patchExpected: true},
		{name: "service-account-default-token", allowed: true, patchExpected: true},
		{name: "volumetest", allowed: true, patchExpected: true},
		{name: "volumetest-existingvolume", allowed: true, patchExpected: true},
	}
	sidecarConfigs, _           = filepath.Glob(path.Join(sidecars, "*.yaml"))
	expectedNumInjectionConfigs = len(sidecarConfigs)
)

type expectedSidecarConfiguration struct {
	configuration   string
	expectedSidecar string
	expectedError   error
}

type mutationTest struct {
	// name is a file relative to test/fixtures/k8s/admissioncontrol/request/ ending in .yaml
	//  which is the v1.AdmissionRequest object passed to mutate
	name          string
	allowed       bool
	patchExpected bool
}

func TestLoadConfig(t *testing.T) {
	c, err := config.LoadConfigDirectory(sidecars)
	if err != nil {
		t.Fatal(err)
	}
	c.AnnotationNamespace = "injector.unittest.com"
	if len(c.Injections) != expectedNumInjectionConfigs {
		t.Fatalf("expected %d injection configs to be loaded from %s, but got %d", expectedNumInjectionConfigs, sidecars, len(c.Injections))
	}
	if c.AnnotationNamespace != "injector.unittest.com" {
		t.Fatalf("expected injector.unittest.com default AnnotationNamespace but got %s", c.AnnotationNamespace)
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
			t.Fatalf("unable to load object metadata yaml: %v", err)
		}

		var obj *metav1.ObjectMeta
		if err := yaml.Unmarshal(data, &obj); err != nil {
			t.Fatalf("unable to unmarshal object metadata yaml: %v", err)
		}
		key, err := s.getSidecarConfigurationRequested(testIgnoredNamespaces, obj)
		if err != test.expectedError {
			t.Fatalf("%s: (expectedSidecar %s) error: %v did not match %v (k %v)", test.configuration, test.expectedSidecar, err, test.expectedError, key)
		}
		if key != test.expectedSidecar {
			t.Fatalf("%s: expected sidecar to be %v but was %v instead", test.configuration, test.expectedSidecar, key)
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
		var req v1.AdmissionRequest
		reqFile := fmt.Sprintf("test/fixtures/k8s/admissioncontrol/request/%s.yaml", test.name)
		resPatchFile := fmt.Sprintf("test/fixtures/k8s/admissioncontrol/patch/%s.json", test.name)
		// load the AdmissionRequest object
		reqData, err := ioutil.ReadFile(reqFile)
		if err != nil {
			t.Fatalf("%s: unable to load AdmissionRequest object: %v", reqFile, err)
		}
		if err := yaml.Unmarshal(reqData, &req); err != nil {
			t.Fatalf("%s: unable to unmarshal AdmissionRequest yaml: %v", reqFile, err)
		}

		// stuff the request into mutate, and catch the response
		res := s.mutate(&req)

		// extract this field, so we can diff json separate from the AdmissionResponse object
		resPatch := res.Patch
		res.Patch = nil // zero this field out

		if test.allowed != res.Allowed {
			t.Fatalf("expected AdmissionResponse.Allowed=%v differed from received AdmissionResponse.Allowed=%v", test.allowed, res.Allowed)
		}

		// diff the JSON patch object with expected JSON loaded from disk
		// we do this because this is way easier on the eyes than diffing
		// a yaml base64 encoded string
		if test.patchExpected {
			if _, err := os.Stat(resPatchFile); err != nil {
				t.Fatalf("%s: unable to load expected patch JSON response: %v", resPatchFile, err)
			}
			t.Logf("Loading patch data from %s...", resPatchFile)
			expectedPatchData, err := ioutil.ReadFile(resPatchFile)
			if err != nil {
				t.Error(err)
				t.Fail()
			}
			difference, diffString := jsondiff.Compare(expectedPatchData, resPatch, &jsondiffopts)
			if difference != jsondiff.FullMatch {
				t.Errorf("Actual patch JSON: %s", string(resPatch))
				t.Fatalf("received AdmissionResponse.patch field differed from expected with %s (%s) (actual on left, expected on right):\n%s", resPatchFile, difference.String(), diffString)
			}
		}

	}
}
