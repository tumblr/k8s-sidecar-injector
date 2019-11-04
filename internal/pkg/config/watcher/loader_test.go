package watcher

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/tumblr/k8s-sidecar-injector/internal/pkg/config"
	testhelper "github.com/tumblr/k8s-sidecar-injector/internal/pkg/testing"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
)

var (
	fixtureSidecarsDir = "test/fixtures/sidecars"
	fixtureK8sDir      = "test/fixtures/k8s"

	// maps a k8s ConfigMap fixture in test/fixtures/k8s/ => testhelper.ConfigExpectation
	ExpectedInjectionConfigFixtures = map[string][]testhelper.ConfigExpectation{
		"configmap-env1": []testhelper.ConfigExpectation{
			testhelper.ConfigExpectation{
				Name:               "env1",
				Version:            "latest",
				Path:               fixtureSidecarsDir + "/env1.yaml",
				VolumeCount:        0,
				EnvCount:           3,
				ContainerCount:     0,
				VolumeMountCount:   0,
				HostAliasCount:     0,
				InitContainerCount: 0,
			},
		},
		"configmap-sidecar-test": []testhelper.ConfigExpectation{
			testhelper.ConfigExpectation{
				Name:               "sidecar-test",
				Version:            "latest",
				Path:               fixtureSidecarsDir + "/sidecar-test.yaml",
				VolumeCount:        1,
				EnvCount:           2,
				ContainerCount:     2,
				VolumeMountCount:   0,
				HostAliasCount:     0,
				InitContainerCount: 0,
			},
		},
		"configmap-complex-sidecar": []testhelper.ConfigExpectation{
			testhelper.ConfigExpectation{
				Name:               "complex-sidecar",
				Version:            "v420.69",
				Path:               fixtureSidecarsDir + "/complex-sidecar.yaml",
				VolumeCount:        1,
				EnvCount:           0,
				ContainerCount:     4,
				VolumeMountCount:   0,
				HostAliasCount:     0,
				InitContainerCount: 0,
			},
		},
		"configmap-multiple1": []testhelper.ConfigExpectation{
			testhelper.ConfigExpectation{
				Name:               "env1",
				Version:            "latest",
				Path:               fixtureSidecarsDir + "/env1.yaml",
				VolumeCount:        0,
				EnvCount:           3,
				ContainerCount:     0,
				VolumeMountCount:   0,
				HostAliasCount:     0,
				InitContainerCount: 0,
			},
			testhelper.ConfigExpectation{
				Name:               "sidecar-test",
				Version:            "latest",
				Path:               fixtureSidecarsDir + "/sidecar-test.yaml",
				VolumeCount:        1,
				EnvCount:           2,
				ContainerCount:     2,
				VolumeMountCount:   0,
				HostAliasCount:     0,
				InitContainerCount: 0,
			},
		},
		"configmap-volume-mounts": []testhelper.ConfigExpectation{
			testhelper.ConfigExpectation{
				Name:               "volume-mounts",
				Version:            "latest",
				Path:               fixtureSidecarsDir + "/volume-mounts.yaml",
				VolumeCount:        2,
				EnvCount:           2,
				ContainerCount:     3,
				VolumeMountCount:   1,
				HostAliasCount:     0,
				InitContainerCount: 0,
			},
		},
		"configmap-host-aliases": []testhelper.ConfigExpectation{
			testhelper.ConfigExpectation{
				Name:               "host-aliases",
				Version:            "latest",
				Path:               fixtureSidecarsDir + "/host-aliases.yaml",
				VolumeCount:        0,
				EnvCount:           2,
				ContainerCount:     1,
				VolumeMountCount:   0,
				HostAliasCount:     6,
				InitContainerCount: 0,
			},
		},
		"configmap-init-containers": []testhelper.ConfigExpectation{
			testhelper.ConfigExpectation{
				Name:               "init-containers",
				Version:            "latest",
				Path:               fixtureSidecarsDir + "/init-containers.yaml",
				VolumeCount:        0,
				EnvCount:           0,
				ContainerCount:     2,
				VolumeMountCount:   0,
				HostAliasCount:     0,
				InitContainerCount: 1,
			},
		},
	}
)

func k8sFixture(f string) string {
	return fmt.Sprintf("%s/%s.yaml", fixtureK8sDir, f)
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
			expectedNames[i] = f.FullName()
		}
		sort.Strings(expectedNames)
		actualNames := []string{}
		for _, x := range ics {
			actualNames = append(actualNames, x.FullName())
		}
		sort.Strings(actualNames)
		if strings.Join(expectedNames, ",") != strings.Join(actualNames, ",") {
			t.Fatalf("expected InjectionConfigs loaded with names %v but got %v", expectedNames, actualNames)
		}

		for _, expectedICF := range expectedFixtures {
			expectedicFile := expectedICF.Path
			ic, err := config.LoadInjectionConfigFromFilePath(expectedicFile)
			if err != nil {
				t.Fatalf("unable to load expected fixture %s: %s", expectedicFile, err.Error())
			}
			if ic.Name != expectedICF.Name {
				t.Fatalf("expected %s Name in %s, but found %s", expectedICF.Name, expectedICF.Path, ic.Name)
			}
			if ic.Version() != expectedICF.Version {
				t.Fatalf("expected %s Version in %s, but found %s", expectedICF.Version, expectedICF.Path, ic.Version())
			}
			if ic.FullName() != expectedICF.FullName() {
				t.Fatalf("expected %s FullName() in %s, but found %s", expectedICF.FullName(), expectedICF.Path, ic.FullName())
			}
			if len(ic.Environment) != expectedICF.EnvCount {
				t.Fatalf("expected %d environment variables in %s, but found %d", expectedICF.EnvCount, expectedICF.Path, len(ic.Environment))
			}
			if len(ic.Containers) != expectedICF.ContainerCount {
				t.Fatalf("expected %d containers in %s, but found %d", expectedICF.ContainerCount, expectedICF.Path, len(ic.Containers))
			}
			if len(ic.Volumes) != expectedICF.VolumeCount {
				t.Fatalf("expected %d volumes in %s, but found %d", expectedICF.VolumeCount, expectedICF.Path, len(ic.Volumes))
			}
			if len(ic.VolumeMounts) != expectedICF.VolumeMountCount {
				t.Fatalf("expected %d volume mounts in %s, but found %d", expectedICF.VolumeMountCount, expectedICF.Path, len(ic.VolumeMounts))
			}
			if len(ic.HostAliases) != expectedICF.HostAliasCount {
				t.Fatalf("expected %d host aliases in %s, but found %d", expectedICF.HostAliasCount, expectedICF.Path, len(ic.HostAliases))
			}
			if len(ic.InitContainers) != expectedICF.InitContainerCount {
				t.Fatalf("expected %d init containers in %s, but found %d", expectedICF.InitContainerCount, expectedICF.Path, len(ic.InitContainers))
			}
			for _, actualIC := range ics {
				if ic.FullName() == actualIC.FullName() {
					if ic.String() != actualIC.String() {
						t.Fatalf("%s: expected %s to equal %s", expectedICF.Path, ic.String(), actualIC.String())
					}
				}
			}
		}
	}
}
