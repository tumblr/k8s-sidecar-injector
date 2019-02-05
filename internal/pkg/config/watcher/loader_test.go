package watcher

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/tumblr/k8s-sidecar-injector/internal/pkg/config"
	_ "github.com/tumblr/k8s-sidecar-injector/internal/pkg/testing"
	"gopkg.in/yaml.v2"
	"k8s.io/api/core/v1"
)

type injectionConfigExpectation struct {
	name             string
	volumeCount      int
	envCount         int
	containerCount   int
	volumeMountCount int
}

var (
	// maps a k8s ConfigMap fixture in test/fixtures/k8s/ => InjectionConfigExpectation
	ExpectedInjectionConfigFixtures = map[string][]injectionConfigExpectation{
		"configmap-env1": []injectionConfigExpectation{
			injectionConfigExpectation{
				name:             "env1",
				volumeCount:      0,
				envCount:         3,
				containerCount:   0,
				volumeMountCount: 0,
			},
		},
		"configmap-sidecar-test": []injectionConfigExpectation{
			injectionConfigExpectation{
				name:             "sidecar-test",
				volumeCount:      1,
				envCount:         2,
				containerCount:   2,
				volumeMountCount: 0,
			},
		},
		"configmap-complex-sidecar": []injectionConfigExpectation{
			injectionConfigExpectation{
				name:             "complex-sidecar",
				volumeCount:      1,
				envCount:         0,
				containerCount:   4,
				volumeMountCount: 0,
			},
		},
		"configmap-multiple1": []injectionConfigExpectation{
			injectionConfigExpectation{
				name:             "env1",
				volumeCount:      0,
				envCount:         3,
				containerCount:   0,
				volumeMountCount: 0,
			},
			injectionConfigExpectation{
				name:             "sidecar-test",
				volumeCount:      1,
				envCount:         2,
				containerCount:   2,
				volumeMountCount: 0,
			},
		},
		"configmap-volume-mounts": []injectionConfigExpectation{
			injectionConfigExpectation{
				name:             "volume-mounts",
				volumeCount:      2,
				envCount:         2,
				containerCount:   3,
				volumeMountCount: 1,
			},
		},
	}
)

func k8sFixture(f string) string {
	return fmt.Sprintf("test/fixtures/k8s/%s.yaml", f)
}

func injectionConfigFixture(e injectionConfigExpectation) string {
	return fmt.Sprintf("test/fixtures/sidecars/%s.yaml", e.name)
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

		// make sure all the appropriate names are present
		expectedNames := make([]string, len(expectedFixtures))
		for i, f := range expectedFixtures {
			expectedNames[i] = f.name
		}
		sort.Strings(expectedNames)
		actualNames := []string{}
		for _, x := range ics {
			actualNames = append(actualNames, x.Name)
		}
		sort.Strings(actualNames)
		if strings.Join(expectedNames, ",") != strings.Join(actualNames, ",") {
			t.Fatalf("expected InjectionConfigs loaded with names %v but got %v", expectedNames, actualNames)
		}

		for _, expectedICF := range expectedFixtures {
			expectedicFile := injectionConfigFixture(expectedICF)
			ic, err := config.LoadInjectionConfigFromFilePath(expectedicFile)
			if err != nil {
				t.Fatalf("unable to load expected fixture %s: %s", expectedicFile, err.Error())
			}
			if len(ic.Environment) != expectedICF.envCount {
				t.Fatalf("expected %d environment variables in %s, but found %d", expectedICF.envCount, expectedICF.name, len(ic.Environment))
			}
			if len(ic.Containers) != expectedICF.containerCount {
				t.Fatalf("expected %d containers in %s, but found %d", expectedICF.containerCount, expectedICF.name, len(ic.Containers))
			}
			if len(ic.Volumes) != expectedICF.volumeCount {
				t.Fatalf("expected %d volumes in %s, but found %d", expectedICF.volumeCount, expectedICF.name, len(ic.Volumes))
			}
			if len(ic.VolumeMounts) != expectedICF.volumeMountCount {
				t.Fatalf("expected %d volume mounts in %s, but found %d", expectedICF.volumeMountCount, expectedICF.name, len(ic.VolumeMounts))
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
