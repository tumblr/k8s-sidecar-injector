package config

import (
	"testing"

	testhelper "github.com/tumblr/k8s-sidecar-injector/internal/pkg/testing"
)

var (
	// location of the fixture sidecar files
	fixtureSidecarsDir = "test/fixtures/sidecars"

	// test files and expectations
	testConfigs = map[string]testhelper.ConfigExpectation{
		"sidecar-test": testhelper.ConfigExpectation{
			Name:               "sidecar-test",
			Version:            "latest",
			Path:               fixtureSidecarsDir + "/sidecar-test.yaml",
			EnvCount:           2,
			ContainerCount:     2,
			VolumeCount:        1,
			VolumeMountCount:   0,
			HostAliasCount:     0,
			InitContainerCount: 0,
		},
		"complex-sidecar": testhelper.ConfigExpectation{
			Name:               "complex-sidecar",
			Version:            "v420.69",
			Path:               fixtureSidecarsDir + "/complex-sidecar.yaml",
			EnvCount:           0,
			ContainerCount:     4,
			VolumeCount:        1,
			VolumeMountCount:   0,
			HostAliasCount:     0,
			InitContainerCount: 0,
		},
		"env1": testhelper.ConfigExpectation{
			Name:               "env1",
			Version:            "latest",
			Path:               fixtureSidecarsDir + "/env1.yaml",
			EnvCount:           3,
			ContainerCount:     0,
			VolumeCount:        0,
			VolumeMountCount:   0,
			HostAliasCount:     0,
			InitContainerCount: 0,
		},
		"volume-mounts": testhelper.ConfigExpectation{
			Name:               "volume-mounts",
			Version:            "latest",
			Path:               fixtureSidecarsDir + "/volume-mounts.yaml",
			EnvCount:           2,
			ContainerCount:     3,
			VolumeCount:        2,
			VolumeMountCount:   1,
			HostAliasCount:     0,
			InitContainerCount: 0,
		},
		"host-aliases": testhelper.ConfigExpectation{
			Name:               "host-aliases",
			Version:            "latest",
			Path:               fixtureSidecarsDir + "/host-aliases.yaml",
			EnvCount:           2,
			ContainerCount:     1,
			VolumeCount:        0,
			VolumeMountCount:   0,
			HostAliasCount:     6,
			InitContainerCount: 0,
		},
		"init-containers": testhelper.ConfigExpectation{
			Name:               "init-containers",
			Version:            "latest",
			Path:               fixtureSidecarsDir + "/init-containers.yaml",
			EnvCount:           0,
			ContainerCount:     2,
			VolumeCount:        0,
			VolumeMountCount:   0,
			HostAliasCount:     0,
			InitContainerCount: 1,
		},
		"versioned1": testhelper.ConfigExpectation{
			Name:               "init-containers",
			Version:            "v2",
			Path:               fixtureSidecarsDir + "/init-containers-v2.yaml",
			EnvCount:           0,
			ContainerCount:     2,
			VolumeCount:        0,
			VolumeMountCount:   0,
			HostAliasCount:     0,
			InitContainerCount: 1,
		},
		// test that a name with spurious use of ":" returns the expected Name and Version
		"versioned:with:extra:data:v3": testhelper.ConfigExpectation{
			Name:               "init-containers:extra:data",
			Version:            "v3",
			Path:               fixtureSidecarsDir + "/init-containers-colons-v3.yaml",
			EnvCount:           0,
			ContainerCount:     2,
			VolumeCount:        0,
			VolumeMountCount:   0,
			HostAliasCount:     0,
			InitContainerCount: 1,
		},
		// test simple inheritance
		"simple inheritance from complex-sidecar": testhelper.ConfigExpectation{
			Name:               "inheritance-complex",
			Version:            "v1",
			Path:               fixtureSidecarsDir + "/inheritance-1.yaml",
			EnvCount:           2,
			ContainerCount:     5,
			VolumeCount:        2,
			VolumeMountCount:   0,
			HostAliasCount:     1,
			InitContainerCount: 1,
		},
		// test deep inheritance
		"deep inheritance from inheritance-complex": testhelper.ConfigExpectation{
			Name:               "inheritance-deep",
			Version:            "v2",
			Path:               fixtureSidecarsDir + "/inheritance-deep-2.yaml",
			EnvCount:           3,
			ContainerCount:     6,
			VolumeCount:        3,
			VolumeMountCount:   0,
			HostAliasCount:     3,
			InitContainerCount: 2,
		},
	}
)

// TestConfigs: load configs from filepath and check if we load what we expected
func TestConfigs(t *testing.T) {
	for _, testConfig := range testConfigs {
		c, err := LoadInjectionConfigFromFilePath(testConfig.Path)
		if err != nil {
			t.Error(err)
			t.Fail()
		}
		if c.Name != testConfig.Name {
			t.Errorf("expected %s Name loaded from %s but got %s", testConfig.Name, testConfig.Path, c.Name)
			t.Fail()
		}
		if c.Version() != testConfig.Version {
			t.Errorf("expected %s Version() loaded from %s but got %s", testConfig.Version, testConfig.Path, c.Version())
			t.Fail()
		}
		if c.FullName() != testConfig.FullName() {
			t.Errorf("expected FullName() %s loaded from %s but got %s", testConfig.FullName(), testConfig.Path, c.FullName())
			t.Fail()
		}
		if len(c.Environment) != testConfig.EnvCount {
			t.Errorf("expected %d Envs loaded from %s but got %d", testConfig.EnvCount, testConfig.Path, len(c.Environment))
			t.Fail()
		}
		if len(c.Containers) != testConfig.ContainerCount {
			t.Errorf("expected %d Containers loaded from %s but got %d", testConfig.ContainerCount, testConfig.Path, len(c.Containers))
			t.Fail()
		}
		if len(c.Volumes) != testConfig.VolumeCount {
			t.Errorf("expected %d Volumes loaded from %s but got %d", testConfig.VolumeCount, testConfig.Path, len(c.Volumes))
			t.Fail()
		}
		if len(c.VolumeMounts) != testConfig.VolumeMountCount {
			t.Fatalf("expected %d VolumeMounts loaded from %s but got %d", testConfig.VolumeMountCount, testConfig.Path, len(c.VolumeMounts))
		}
		if len(c.HostAliases) != testConfig.HostAliasCount {
			t.Fatalf("expected %d HostAliases loaded from %s but got %d", testConfig.HostAliasCount, testConfig.Path, len(c.HostAliases))
		}
		if len(c.InitContainers) != testConfig.InitContainerCount {
			t.Fatalf("expected %d InitContainers loaded from %s but got %d", testConfig.InitContainerCount, testConfig.Path, len(c.InitContainers))
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

	if i.Name != cfg.Name {
		t.Fatalf("expected name %s, but got %s", cfg.Name, i.Name)
	}
	if i.Version() != cfg.Version {
		t.Fatalf("expected version %s, but got %s", cfg.Version, i.Version())
	}
	if i.FullName() != cfg.FullName() {
		t.Fatalf("expected FullName %s, but got %s", cfg.FullName(), i.FullName())
	}
	if len(i.Environment) != cfg.EnvCount {
		t.Fatalf("expected %d Envs, but got %d", cfg.EnvCount, len(i.Environment))
	}
	if len(i.Containers) != cfg.ContainerCount {
		t.Fatalf("expected %d container, but got %d", cfg.ContainerCount, len(i.Containers))
	}
	if len(i.Volumes) != cfg.VolumeCount {
		t.Fatalf("expected %d volume, but got %d", cfg.VolumeCount, len(i.Volumes))
	}
	if len(i.VolumeMounts) != cfg.VolumeMountCount {
		t.Fatalf("expected %d VolumeMounts, but got %d", cfg.VolumeMountCount, len(i.VolumeMounts))
	}
	if len(i.HostAliases) != cfg.HostAliasCount {
		t.Fatalf("expected %d HostAliases, but got %d", cfg.HostAliasCount, len(i.HostAliases))
	}
	if len(i.InitContainers) != cfg.InitContainerCount {
		t.Fatalf("expected %d InitContainers, but got %d", cfg.InitContainerCount, len(i.InitContainers))
	}
}
