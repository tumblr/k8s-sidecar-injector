package config

import (
	"fmt"
	"testing"

	testhelper "github.com/tumblr/k8s-sidecar-injector/internal/pkg/testing"
)

var (
	// location of the fixture sidecar files
	fixtureSidecarsDir = "test/fixtures/sidecars"

	testBadConfigs = map[string]testhelper.ConfigExpectation{
		// test that a name with spurious use of ":" errors out on load
		"versioned:with:extra:data:v3": testhelper.ConfigExpectation{
			Path:      fixtureSidecarsDir + "/bad/init-containers-colons-v3.yaml",
			LoadError: ErrUnsupportedNameVersionFormat,
		},
		"missing name": testhelper.ConfigExpectation{
			Path:      fixtureSidecarsDir + "/bad/missing-name.yaml",
			LoadError: ErrMissingName,
		},
		"inheritance filenotfound": testhelper.ConfigExpectation{
			Path:      fixtureSidecarsDir + "/bad/inheritance-filenotfound.yaml",
			LoadError: fmt.Errorf(`error loading injection config from file test/fixtures/sidecars/bad/some-missing-file.yaml: open test/fixtures/sidecars/bad/some-missing-file.yaml: no such file or directory`),
		},
		"inheritance escape": testhelper.ConfigExpectation{
			Path:      fixtureSidecarsDir + "/bad/inheritance-escape.yaml",
			LoadError: fmt.Errorf(`error loading injection config from file test/fixtures/etc/passwd: open test/fixtures/etc/passwd: no such file or directory`),
		},
	}

	// test files and expectations
	testGoodConfigs = map[string]testhelper.ConfigExpectation{
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
		"service-account": testhelper.ConfigExpectation{
			Name:               "service-account",
			Version:            "latest",
			Path:               fixtureSidecarsDir + "/service-account.yaml",
			EnvCount:           0,
			ContainerCount:     0,
			VolumeCount:        0,
			VolumeMountCount:   0,
			HostAliasCount:     0,
			InitContainerCount: 0,
			ServiceAccount:     "someaccount",
		},
		// we found that inheritance could cause the loading of the ServiceAccount
		// to fail, so we test explicitly for this case.
		"service-account-with-inheritance": testhelper.ConfigExpectation{
			Name:               "service-account-inherits-env1",
			Version:            "latest",
			Path:               fixtureSidecarsDir + "/service-account-with-inheritance.yaml",
			EnvCount:           3,
			ContainerCount:     0,
			VolumeCount:        0,
			VolumeMountCount:   0,
			HostAliasCount:     0,
			InitContainerCount: 0,
			ServiceAccount:     "someaccount",
		},
		// also, if we inject a serviceAccount and any container has a VolumeMount
		// with a mountPath of /var/run/secrets/kubernetes.io/serviceaccount, we
		// must remove it, to allow the ServiceAccountController to inject the
		// appropriate token volume
		"service-account-default-token": testhelper.ConfigExpectation{
			Name:               "service-account-default-token",
			Version:            "latest",
			Path:               fixtureSidecarsDir + "/service-account-default-token.yaml",
			EnvCount:           0,
			ContainerCount:     0,
			VolumeCount:        0,
			VolumeMountCount:   0,
			HostAliasCount:     0,
			InitContainerCount: 0,
			ServiceAccount:     "someaccount",
		},
		"maxmind": testhelper.ConfigExpectation{
			Name:               "maxmind",
			Version:            "latest",
			Path:               fixtureSidecarsDir + "/maxmind.yaml",
			EnvCount:           1,
			ContainerCount:     1,
			VolumeCount:        2,
			VolumeMountCount:   1,
			HostAliasCount:     0,
			InitContainerCount: 1,
		},
	}
)

func TestConfigsLoadErrors(t *testing.T) {
	for _, testConfig := range testBadConfigs {
		_, err := LoadInjectionConfigFromFilePath(testConfig.Path)
		if err == nil || testConfig.LoadError == nil {
			t.Fatal("error was nil or LoadError was nil - this test should only be testing load errors")
		}
		if testConfig.LoadError.Error() != err.Error() {
			t.Fatalf("expected %s load to produce error %v but got %v", testConfig.Path, testConfig.LoadError, err)
		}
	}
}

// TestGoodConfigs: load configs from filepath and check if we load what we expected
func TestGoodConfigs(t *testing.T) {
	for _, testConfig := range testGoodConfigs {
		c, err := LoadInjectionConfigFromFilePath(testConfig.Path)
		if testConfig.LoadError != err {
			t.Fatalf("expected %s load to produce error %v but got %v", testConfig.Path, testConfig.LoadError, err)
		}
		if testConfig.LoadError != nil {
			// if we expected a load error, and we made it here, continue, because we do not need to test
			// anything about the loaded InjectionConfig
			continue
		}
		if c.Name != testConfig.Name {
			t.Fatalf("expected %s Name loaded from %s but got %s", testConfig.Name, testConfig.Path, c.Name)
		}
		if c.Version() != testConfig.Version {
			t.Fatalf("expected %s Version() loaded from %s but got %s", testConfig.Version, testConfig.Path, c.Version())
		}
		if c.FullName() != testConfig.FullName() {
			t.Fatalf("expected FullName() %s loaded from %s but got %s", testConfig.FullName(), testConfig.Path, c.FullName())
		}
		if len(c.Environment) != testConfig.EnvCount {
			t.Fatalf("expected %d Envs loaded from %s but got %d", testConfig.EnvCount, testConfig.Path, len(c.Environment))
		}
		if len(c.Containers) != testConfig.ContainerCount {
			t.Fatalf("expected %d Containers loaded from %s but got %d", testConfig.ContainerCount, testConfig.Path, len(c.Containers))
		}
		if len(c.Volumes) != testConfig.VolumeCount {
			t.Fatalf("expected %d Volumes loaded from %s but got %d", testConfig.VolumeCount, testConfig.Path, len(c.Volumes))
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
		if c.ServiceAccountName != testConfig.ServiceAccount {
			t.Fatalf("expected ServiceAccountName %s, but got %s", testConfig.ServiceAccount, c.ServiceAccountName)
		}
	}
}

// TestLoadConfig: Check if we get all the configs
func TestLoadConfig(t *testing.T) {
	expectedNumInjectionsConfig := len(testGoodConfigs)
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
	cfg := testGoodConfigs["sidecar-test"]
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
	if i.ServiceAccountName != cfg.ServiceAccount {
		t.Fatalf("expected ServiceAccountName %s, but got %s", cfg.ServiceAccount, i.ServiceAccountName)
	}
}
