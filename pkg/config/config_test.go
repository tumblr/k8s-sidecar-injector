package config

import (
	"testing"
)

var (
	sidecars          = "../../test/fixtures/sidecars"
	cfg1              = "../../test/fixtures/sidecars/sidecar-test.yaml"
	complicatedConfig = "../../test/fixtures/sidecars/complex-sidecar.yaml"
)

func TestLoadConfig(t *testing.T) {
	expectedNumInjectionsConfig := 3
	c, err := LoadConfigDirectory(sidecars)
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

func TestLoadInjectionConfig1(t *testing.T) {
	cfg := cfg1
	c, err := LoadInjectionConfigFromFilePath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "sidecar-test" {
		t.Fatalf("expected %s Name loaded from %s but got %s", "sidecar-test", cfg, c.Name)
	}
	expectedEnvVars := 2
	if len(c.Environment) != expectedEnvVars {
		t.Fatalf("expected %d EnvVars loaded from %s but got %d", expectedEnvVars, cfg, len(c.Environment))
	}
	if len(c.Containers) != 2 {
		t.Fatalf("expected %d Containers loaded from %s but got %d", 2, cfg, len(c.Containers))
	}
	if len(c.Volumes) != 1 {
		t.Fatalf("expected %d Volumes loaded from %s but got %d", 1, cfg, len(c.Volumes))
	}
}

// load a more complicated test config with LOTS of configuration
func TestLoadComplexConfig(t *testing.T) {
	cfg := complicatedConfig
	c, err := LoadInjectionConfigFromFilePath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	expectedName := "complex-sidecar"
	nExpectedContainers := 4
	nExpectedVolumes := 1
	nExpectedEnvironmentVars := 0
	if c.Name != expectedName {
		t.Fatalf("expected %s Name loaded from %s but got %s", expectedName, cfg, c.Name)
	}
	if len(c.Environment) != nExpectedEnvironmentVars {
		t.Fatalf("expected %d EnvVars loaded from %s but got %d", nExpectedEnvironmentVars, cfg, len(c.Environment))
	}
	if len(c.Containers) != nExpectedContainers {
		t.Fatalf("expected %d Containers loaded from %s but got %d", nExpectedContainers, cfg, len(c.Containers))
	}
	if len(c.Volumes) != nExpectedVolumes {
		t.Fatalf("expected %d Volumes loaded from %s but got %d", nExpectedVolumes, cfg, len(c.Volumes))
	}
}

func TestHasInjectionConfig(t *testing.T) {
	c, err := LoadConfigDirectory(sidecars)
	if err != nil {
		t.Fatal(err)
	}

	for _, k := range []string{"sidecar-test", "complex-sidecar"} {
		if !c.HasInjectionConfig(k) {
			t.Fatalf("%s should have included %s but did not", cfg1, k)
		}
	}

	// make sure it says when things arent present too
	for _, k := range []string{"missing-1", "yolo420blazeit"} {
		if c.HasInjectionConfig(k) {
			t.Fatalf("%s should NOT have included %s but did", cfg1, k)
		}
	}

}

func TestGetInjectionConfig(t *testing.T) {
	c, err := LoadConfigDirectory(sidecars)
	if err != nil {
		t.Fatal(err)
	}

	i, err := c.GetInjectionConfig("sidecar-test")
	if err != nil {
		t.Fatal(err)
	}

	if len(i.Environment) != 2 {
		t.Fatalf("expected 2 envvars, but got %d", len(i.Environment))
	}
	if len(i.Containers) != 2 {
		t.Fatalf("expected 2 container, but got %d", len(i.Containers))
	}
	if len(i.Volumes) != 1 {
		t.Fatalf("expected 1 volume, but got %d", len(i.Volumes))
	}
}
