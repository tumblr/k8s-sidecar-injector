package watcher

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/tumblr/k8s-sidecar-injector/pkg/config"
	"gopkg.in/yaml.v2"
	"k8s.io/api/core/v1"
)

var (
	// maps a k8s ConfigMap fixture in ../../../test/fixtures/k8s/ =>
	// InjectionConfig fixtures in ../../../test/fixtures/sidecars/
	ExpectedInjectionConfigFixtures = map[string][]string{
		"configmap-env1":            []string{"env1"},
		"configmap-sidecar-test":    []string{"sidecar-test"},
		"configmap-complex-sidecar": []string{"complex-sidecar"},
		"configmap-multiple1":       []string{"env1", "sidecar-test"},
	}
)

func k8sFixture(f string) string {
	return fmt.Sprintf("../../../test/fixtures/k8s/%s.yaml", f)
}

func injectionConfigFixture(f string) string {
	return fmt.Sprintf("../../../test/fixtures/sidecars/%s.yaml", f)
}

func TestLoadFromConfigMap(t *testing.T) {
	for fixture, expectedFixtures := range ExpectedInjectionConfigFixtures {
		fname := k8sFixture(fixture)
		t.Logf("loading injection config from %s", fname)
		var cm v1.ConfigMap
		f, err := os.Open(fname)
		if err != nil {
			t.Fatal(err)
		}
		data, err := ioutil.ReadAll(f)
		if err != nil {
			t.Fatal(err)
		}
		if err = yaml.Unmarshal(data, &cm); err != nil {
			t.Fatal(err)
		}

		ics, err := InjectionConfigsFromConfigMap(cm)
		if err != nil {
			t.Fatal(err)
		}
		if len(ics) != len(expectedFixtures) {
			t.Fatalf("expected %d injection configs loaded from %s, but got %d", len(expectedFixtures), fname, len(ics))
		}
		sort.Strings(expectedFixtures)
		actualNames := []string{}
		for _, x := range ics {
			actualNames = append(actualNames, x.Name)
		}
		sort.Strings(actualNames)
		if strings.Join(expectedFixtures, ",") != strings.Join(actualNames, ",") {
			t.Fatalf("expected InjectionConfigs loaded with names %v but got %v", expectedFixtures, actualNames)
		}

		for _, expectedICF := range expectedFixtures {
			expectedicFile := injectionConfigFixture(expectedICF)
			ic, err := config.LoadInjectionConfigFromFilePath(expectedicFile)
			if err != nil {
				t.Fatalf("unable to load expected fixture %s: %s", expectedicFile, err.Error())
			}
			for _, actualIC := range ics {
				if ic.Name == actualIC.Name {
					if ic.String() != actualIC.String() {
						t.Fatalf("expected %s to equal %s", ic.String(), actualIC.String())
					}
				}
			}
		}
	}
}
