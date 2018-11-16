package main

import (
	"fmt"
	"strings"
)

// MapStringStringFlag is a flag struct for key=value pairs
type MapStringStringFlag struct {
	Values map[string]string
}

// String implements the flag.Var interface
func (s *MapStringStringFlag) String() string {
	z := []string{}
	for x, y := range s.Values {
		z = append(z, fmt.Sprintf("%s=%s", x, y))
	}
	return strings.Join(z, ",")
}

// Set implements the flag.Var interface
func (s *MapStringStringFlag) Set(value string) error {
	if s.Values == nil {
		s.Values = map[string]string{}
	}
	for _, p := range strings.Split(value, ",") {
		fields := strings.Split(p, "=")
		if len(fields) != 2 {
			return fmt.Errorf("%s is incorrectly formatted! should be key=value[,key2=value2]", p)
		}
		s.Values[fields[0]] = fields[1]
	}
	return nil
}

// ToMapStringString returns the underlying representation of the map of key=value pairs
func (s *MapStringStringFlag) ToMapStringString() map[string]string {
	return s.Values
}

// NewMapStringStringFlag creates a new flag var for storing key=value pairs
func NewMapStringStringFlag() MapStringStringFlag {
	return MapStringStringFlag{Values: map[string]string{}}
}
