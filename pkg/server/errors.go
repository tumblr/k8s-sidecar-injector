package server

import (
	"fmt"
)

var (
	// ErrSkipIgnoredNamespace ...
	ErrSkipIgnoredNamespace = fmt.Errorf("Skipping pod in ignored namespace")
	// ErrSkipAlreadyInjected ...
	ErrSkipAlreadyInjected = fmt.Errorf("Skipping pod that has already been injected")
	// ErrMissingRequestAnnotation ...
	ErrMissingRequestAnnotation = fmt.Errorf("Missing injection request annotation")
	// ErrRequestedSidecarNotFound ...
	ErrRequestedSidecarNotFound = fmt.Errorf("Requested sidecar not found in configuration")
)
