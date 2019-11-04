package config

import (
	"fmt"
	"strings"
	"testing"

	_ "github.com/tumblr/k8s-sidecar-injector/internal/pkg/testing"
)

// config struct for testing: where to find the file and what we expect to find in it
type configExpectation struct {
	// name is not the Name in the loaded config, but only the "some-config" of "some-config:1.2"
	name string
	// version is the parsed version string, or "latest" if omitted
	version string
	// fullName is the canonical full name, i.e. some-config:1.2
	fullName                   string
	path                       string
	expectedEnvVarCount        int
	expectedContainerCount     int
	expectedVolumeCount        int
	expectedVolumeMountCount   int
	expectedHostAliasCount     int
	expectedInitContainerCount int
}

func (x *configExpectation) FullName() string {
	return strings.ToLower(fmt.Sprintf("%s:%s", x.name, x.version))
}

var (
	// location of the fixture sidecar files
	fixtureSidecarsDir = "test/fixtures/sidecars"

	// test files and expectations
	testConfigs = map[string]configExpectation{
		"sidecar-test": configExpectation{
			name:                       "sidecar-test",
			version:                    "latest",
			path:                       fixtureSidecarsDir + "/sidecar-test.yaml",
			expectedEnvVarCount:        2,
			expectedContainerCount:     2,
			expectedVolumeCount:        1,
			expectedVolumeMountCount:   0,
			expectedHostAliasCount:     0,
			expectedInitContainerCount: 0,
		},
		"complex-sidecar": configExpectation{
			name:                       "complex-sidecar",
			version:                    "latest",
			path:                       fixtureSidecarsDir + "/complex-sidecar.yaml",
			expectedEnvVarCount:        0,
			expectedContainerCount:     4,
			expectedVolumeCount:        1,
			expectedVolumeMountCount:   0,
			expectedHostAliasCount:     0,
			expectedInitContainerCount: 0,
		},
		"env1": configExpectation{
			name:                       "env1",
			version:                    "latest",
			path:                       fixtureSidecarsDir + "/env1.yaml",
			expectedEnvVarCount:        3,
			expectedContainerCount:     0,
			expectedVolumeCount:        0,
			expectedVolumeMountCount:   0,
			expectedHostAliasCount:     0,
			expectedInitContainerCount: 0,
		},
		"volume-mounts": configExpectation{
			name:                       "volume-mounts",
			version:                    "latest",
			path:                       fixtureSidecarsDir + "/volume-mounts.yaml",
			expectedEnvVarCount:        2,
			expectedContainerCount:     3,
			expectedVolumeCount:        2,
			expectedVolumeMountCount:   1,
			expectedHostAliasCount:     0,
			expectedInitContainerCount: 0,
		},
		"host-aliases": configExpectation{
			name:                       "host-aliases",
			version:                    "latest",
			path:                       fixtureSidecarsDir + "/host-aliases.yaml",
			expectedEnvVarCount:        2,
			expectedContainerCount:     1,
			expectedVolumeCount:        0,
			expectedVolumeMountCount:   0,
			expectedHostAliasCount:     6,
			expectedInitContainerCount: 0,
		},
		"init-containers": configExpectation{
			name:                       "init-containers",
			version:                    "latest",
			path:                       fixtureSidecarsDir + "/init-containers.yaml",
			expectedEnvVarCount:        0,
			expectedContainerCount:     2,
			expectedVolumeCount:        0,
			expectedVolumeMountCount:   0,
			expectedHostAliasCount:     0,
			expectedInitContainerCount: 1,
		},
		"versioned1": configExpectation{
			name:                       "init-containers",
			version:                    "v2",
			path:                       fixtureSidecarsDir + "/init-containers-v2.yaml",
			expectedEnvVarCount:        0,
			expectedContainerCount:     2,
			expectedVolumeCount:        0,
			expectedVolumeMountCount:   0,
			expectedHostAliasCount:     0,
			expectedInitContainerCount: 1,
		},
		// test that a name with spurious use of ":" returns the expected Name and Version
		"versioned:with:extra:data:v3": configExpectation{
			name:                       "init-containers:extra:data",
			version:                    "v3",
			path:                       fixtureSidecarsDir + "/init-containers-colons-v3.yaml",
			expectedEnvVarCount:        0,
			expectedContainerCount:     2,
			expectedVolumeCount:        0,
			expectedVolumeMountCount:   0,
			expectedHostAliasCount:     0,
			expectedInitContainerCount: 1,
		},
	}
)

// TestConfigs: load configs from filepath and check if we load what we expected
func TestConfigs(t *testing.T) {
	for _, testConfig := range testConfigs {
		c, err := LoadInjectionConfigFromFilePath(testConfig.path)
		if err != nil {
			t.Error(err)
			t.Fail()
		}
		if c.Name != testConfig.name {
			t.Errorf("expected %s Name loaded from %s but got %s", testConfig.name, testConfig.path, c.Name)
			t.Fail()
		}
		if c.Version() != testConfig.version {
			t.Errorf("expected %s Version() loaded from %s but got %s", testConfig.version, testConfig.path, c.Version())
			t.Fail()
		}
		if c.FullName() != testConfig.FullName() {
			t.Errorf("expected FullName() %s loaded from %s but got %s", testConfig.FullName(), testConfig.path, c.FullName())
			t.Fail()
		}
		if len(c.Environment) != testConfig.expectedEnvVarCount {
			t.Errorf("expected %d EnvVars loaded from %s but got %d", testConfig.expectedEnvVarCount, testConfig.path, len(c.Environment))
			t.Fail()
		}
		if len(c.Containers) != testConfig.expectedContainerCount {
			t.Errorf("expected %d Containers loaded from %s but got %d", testConfig.expectedContainerCount, testConfig.path, len(c.Containers))
			t.Fail()
		}
		if len(c.Volumes) != testConfig.expectedVolumeCount {
			t.Errorf("expected %d Volumes loaded from %s but got %d", testConfig.expectedVolumeCount, testConfig.path, len(c.Volumes))
			t.Fail()
		}
		if len(c.VolumeMounts) != testConfig.expectedVolumeMountCount {
			t.Fatalf("expected %d VolumeMounts loaded from %s but got %d", testConfig.expectedVolumeMountCount, testConfig.path, len(c.VolumeMounts))
		}
		if len(c.HostAliases) != testConfig.expectedHostAliasCount {
			t.Fatalf("expected %d HostAliases loaded from %s but got %d", testConfig.expectedHostAliasCount, testConfig.path, len(c.HostAliases))
		}
		if len(c.InitContainers) != testConfig.expectedInitContainerCount {
			t.Fatalf("expected %d InitContainers loaded from %s but got %d", testConfig.expectedInitContainerCount, testConfig.path, len(c.InitContainers))
		}
	}
}

// TestLoadConfig: Check if we get all the configs
func TestLoadConfig(t *testing.T) {
	expectedNumInjectionsConfig := len(testConfigs)
	c, err := LoadConfigDirectory(fixtureSidecarsDir)
	if err != nil {
		t.Fatal(err)
	}
	if c.AnnotationNamespace != "injector.tumblr.com" {
		t.Fatalf("expected %s AnnotationNamespace but got %s", "injector.tumblr.com", c.AnnotationNamespace)
	}
	if len(c.Injections) != expectedNumInjectionsConfig {
		t.Fatalf("expected %d Injections loaded but got %d", expectedNumInjectionsConfig, len(c.Injections))
	}
}

// TestFetInjectionConfig: Check if we can properly load a config by name and see if we read the correct values from it
func TestGetInjectionConfig(t *testing.T) {
	cfg := testConfigs["sidecar-test"]
	c, err := LoadConfigDirectory(fixtureSidecarsDir)
	if err != nil {
		t.Fatal(err)
	}

	i, err := c.GetInjectionConfig(cfg.FullName())
	if err != nil {
		t.Fatal(err)
	}

	if i.Name != cfg.name {
		t.Fatalf("expected name %s, but got %s", cfg.name, i.Name)
	}
	if i.Version() != cfg.version {
		t.Fatalf("expected version %s, but got %s", cfg.version, i.Version())
	}
	if i.FullName() != cfg.FullName() {
		t.Fatalf("expected FullName %s, but got %s", cfg.FullName(), i.FullName())
	}
	if len(i.Environment) != cfg.expectedEnvVarCount {
		t.Fatalf("expected %d envvars, but got %d", cfg.expectedEnvVarCount, len(i.Environment))
	}
	if len(i.Containers) != cfg.expectedContainerCount {
		t.Fatalf("expected %d container, but got %d", cfg.expectedContainerCount, len(i.Containers))
	}
	if len(i.Volumes) != cfg.expectedVolumeCount {
		t.Fatalf("expected %d volume, but got %d", cfg.expectedVolumeCount, len(i.Volumes))
	}
	if len(i.VolumeMounts) != cfg.expectedVolumeMountCount {
		t.Fatalf("expected %d VolumeMounts, but got %d", cfg.expectedVolumeMountCount, len(i.VolumeMounts))
	}
	if len(i.HostAliases) != cfg.expectedHostAliasCount {
		t.Fatalf("expected %d HostAliases, but got %d", cfg.expectedHostAliasCount, len(i.HostAliases))
	}
	if len(i.InitContainers) != cfg.expectedInitContainerCount {
		t.Fatalf("expected %d InitContainers, but got %d", cfg.expectedInitContainerCount, len(i.InitContainers))
	}
}
