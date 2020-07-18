package testing

import (
	"fmt"
	"strings"
)

// ConfigExpectation struct for testing: where to find the file and what we expect to find in it
type ConfigExpectation struct {
	// name is not the Name in the loaded config, but only the "some-config" of "some-config:1.2"
	Name string
	// version is the parsed version string, or "latest" if omitted
	Version string
	// Path is the path to the YAML to load the sidecar yaml from
	Path               string
	EnvCount           int
	ContainerCount     int
	VolumeCount        int
	VolumeMountCount   int
	HostAliasCount     int
	InitContainerCount int
	ServiceAccount     string

	// LoadError is an error, if any, that is expected during load
	LoadError error
}

func (x *ConfigExpectation) FullName() string {
	return strings.ToLower(fmt.Sprintf("%s:%s", x.Name, x.Version))
}
