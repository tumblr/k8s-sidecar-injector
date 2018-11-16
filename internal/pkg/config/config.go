package config

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
)

const (
	annotationNamespaceDefault = "injector.tumblr.com"
)

var (
	// InjectionStatusAnnotation is the annotation set on resources to reflect the status of injection
	InjectionStatusAnnotation = fmt.Sprintf("%s/status", annotationNamespaceDefault)
	// ErrMissingName ..
	ErrMissingName = fmt.Errorf(`name field is required for an injection config`)
	// ErrNoConfigurationLoaded ..
	ErrNoConfigurationLoaded = fmt.Errorf(`at least one config must be present in the --config-directory`)
)

// InjectionConfig is a specific instance of a injected config, for a given annotation
type InjectionConfig struct {
	Name        string             `yaml:"name"`
	Containers  []corev1.Container `yaml:"containers"`
	Volumes     []corev1.Volume    `yaml:"volumes"`
	Environment []corev1.EnvVar    `yaml:"env"`
}

// Config is a struct indicating how a given injection should be configured
type Config struct {
	sync.RWMutex
	AnnotationNamespace string                     `yaml:"annotationnamespace"`
	Injections          map[string]InjectionConfig `yaml:"injections"`
}

// String returns a string representation of the config
func (c *InjectionConfig) String() string {
	return fmt.Sprintf("%s: %d containers, %d volumes, %d environment vars", c.Name, len(c.Containers), len(c.Volumes), len(c.Environment))
}

// ReplaceInjectionConfigs will take a list of new InjectionConfigs, and replace the current configuration with them.
// this blocks waiting on being able to update the configs in place.
func (c *Config) ReplaceInjectionConfigs(replacementConfigs []InjectionConfig) {
	c.Lock()
	defer c.Unlock()
	c.Injections = map[string]InjectionConfig{}
	for _, r := range replacementConfigs {
		c.Injections[r.Name] = r
	}
}

// HasInjectionConfig returns bool for whether the config contains a config
// given some key identifier
func (c *Config) HasInjectionConfig(key string) bool {
	c.RLock()
	defer c.RUnlock()
	_, ok := c.Injections[key]
	return ok
}

// GetInjectionConfig returns the InjectionConfig given a requested key
func (c *Config) GetInjectionConfig(key string) (*InjectionConfig, error) {
	c.RLock()
	defer c.RUnlock()
	i, ok := c.Injections[key]
	if !ok {
		return nil, fmt.Errorf("no injection config found for annotation %s", key)
	}
	return &i, nil
}

// LoadConfigDirectory loads all configs in a directory and returns the Config
func LoadConfigDirectory(path string) (*Config, error) {
	cfg := Config{
		Injections: map[string]InjectionConfig{},
	}
	glob := filepath.Join(path, "*.yaml")
	matches, err := filepath.Glob(glob)
	if err != nil {
		return nil, err
	}
	for _, p := range matches {
		c, err := LoadInjectionConfigFromFilePath(p)
		if err != nil {
			glog.Errorf("Error reading injection config from %s: %v", p, err)
			return nil, err
		}
		cfg.Injections[c.Name] = *c
	}

	if len(cfg.Injections) == 0 {
		return nil, ErrNoConfigurationLoaded
	}

	if cfg.AnnotationNamespace == "" {
		cfg.AnnotationNamespace = annotationNamespaceDefault
	}

	glog.V(2).Infof("Loaded %d injection configs from %s", len(cfg.Injections), glob)

	return &cfg, nil
}

// LoadInjectionConfigFromFilePath returns a InjectionConfig given a yaml file on disk
func LoadInjectionConfigFromFilePath(configFile string) (*InjectionConfig, error) {
	f, err := os.Open(configFile)
	defer f.Close()
	if err != nil {
		return nil, fmt.Errorf("error loading injection config from file %s: %s", configFile, err.Error())
	}
	glog.V(3).Infof("Loading injection config from file %s", configFile)
	return LoadInjectionConfig(f)
}

// LoadInjectionConfig takes an io.Reader and parses out an injectionconfig
func LoadInjectionConfig(reader io.Reader) (*InjectionConfig, error) {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var cfg InjectionConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.Name == "" {
		return nil, ErrMissingName
	}

	glog.V(3).Infof("Loaded injection config %s sha256sum=%x", cfg.Name, sha256.Sum256(data))

	return &cfg, nil
}
