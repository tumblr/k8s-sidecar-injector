package config

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
)

const (
	annotationNamespaceDefault = "injector.tumblr.com"
	defaultVersion             = "latest"
)

var (
	// ErrMissingName ..
	ErrMissingName = fmt.Errorf(`name field is required for an injection config`)
	// ErrNoConfigurationLoaded ..
	ErrNoConfigurationLoaded = fmt.Errorf(`at least one config must be present in the --config-directory`)
)

// InjectionConfig is a specific instance of a injected config, for a given annotation
type InjectionConfig struct {
	Name           string               `json:"name"`
	Containers     []corev1.Container   `json:"containers"`
	Volumes        []corev1.Volume      `json:"volumes"`
	Environment    []corev1.EnvVar      `json:"env"`
	VolumeMounts   []corev1.VolumeMount `json:"volumeMounts"`
	HostAliases    []corev1.HostAlias   `json:"hostAliases"`
	InitContainers []corev1.Container   `json:"initContainers"`

	version string
}

// Config is a struct indicating how a given injection should be configured
type Config struct {
	sync.RWMutex
	AnnotationNamespace string                      `yaml:"annotationnamespace"`
	Injections          map[string]*InjectionConfig `yaml:"injections"`
}

// String returns a string representation of the config
func (c *InjectionConfig) String() string {
	return fmt.Sprintf("%s: %d containers, %d init containers, %d volumes, %d environment vars, %d volume mounts, %d host aliases", c.FullName(), len(c.Containers), len(c.InitContainers), len(c.Volumes), len(c.Environment), len(c.VolumeMounts), len(c.HostAliases))
}

// Version returns the parsed version of this injection config. If no version is specified,
// "latest" is returned. The version is extracted from the request annotation, i.e.
// injector.tumblr.com/request: my-sidecar:1.2, where "1.2" is the version.
func (c *InjectionConfig) Version() string {
	if c.version == "" {
		return defaultVersion
	}

	return c.version
}

// FullName returns the full identifier of this sidecar - both the Name, and the Version(), formatted like
// "${.Name}:${.Version}"
func (c *InjectionConfig) FullName() string {
	return canonicalizeConfigName(c.Name, c.Version())
}

// ReplaceInjectionConfigs will take a list of new InjectionConfigs, and replace the current configuration with them.
// this blocks waiting on being able to update the configs in place.
func (c *Config) ReplaceInjectionConfigs(replacementConfigs []*InjectionConfig) {
	c.Lock()
	defer c.Unlock()
	c.Injections = map[string]*InjectionConfig{}

	for _, r := range replacementConfigs {
		c.Injections[r.FullName()] = r
	}
}

// HasInjectionConfig returns bool for whether the config contains a config
// given some key identifier
func (c *Config) HasInjectionConfig(key string) bool {
	c.RLock()
	defer c.RUnlock()

	name, version := configNameFields(key)
	fullKey := canonicalizeConfigName(name, version)

	_, ok := c.Injections[fullKey]

	return ok
}

// GetInjectionConfig returns the InjectionConfig given a requested key
func (c *Config) GetInjectionConfig(key string) (*InjectionConfig, error) {
	c.RLock()
	defer c.RUnlock()

	name, version := configNameFields(key)
	fullKey := canonicalizeConfigName(name, version)

	i, ok := c.Injections[fullKey]
	if !ok {
		return nil, fmt.Errorf("no injection config found for annotation %s", fullKey)
	}

	return i, nil
}

// LoadConfigDirectory loads all configs in a directory and returns the Config
func LoadConfigDirectory(path string) (*Config, error) {
	cfg := Config{
		Injections: map[string]*InjectionConfig{},
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

		cfg.Injections[c.FullName()] = c
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
	if err != nil {
		return nil, fmt.Errorf("error loading injection config from file %s: %s", configFile, err.Error())
	}

	defer f.Close()
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

	// we need to split the Name field apart into a Name and Version component
	cfg.Name, cfg.version = configNameFields(cfg.Name)

	glog.V(3).Infof("Loaded injection config %s version=%s sha256sum=%x", cfg.Name, cfg.Version(), sha256.Sum256(data))

	return &cfg, nil
}

// given a name of a config, extract the name and version. Format is "name[:version]" where :version
// is optional, and is assumed to be "latest" if omitted.
func configNameFields(shortName string) (name, version string) {
	substrings := strings.Split(shortName, ":")

	if len(substrings) <= 1 {
		// no :<version> specified, so assume default version
		return shortName, defaultVersion
	}

	versionField := len(substrings) - 1

	if substrings[versionField] == "" {
		return strings.Join(substrings[:versionField], ":"), defaultVersion
	}

	return strings.Join(substrings[:versionField], ":"), substrings[versionField]
}

func canonicalizeConfigName(name, version string) string {
	return strings.ToLower(fmt.Sprintf("%s:%s", name, version))
}
