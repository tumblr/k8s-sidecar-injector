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

// GetErrorReason returns a string description for a given error, for use
// when reporting "reason" in metrics
func GetErrorReason(err error) string {
	var reason string
	switch err {
	case ErrSkipIgnoredNamespace:
		reason = "ignored_namespace"
	case ErrSkipAlreadyInjected:
		reason = "already_injected"
	case ErrMissingRequestAnnotation:
		reason = "no_annotation"
	case ErrRequestedSidecarNotFound:
		reason = "missing_config"
	case nil:
		reason = ""
	default:
		reason = "unknown_error"
	}
	return reason
}
