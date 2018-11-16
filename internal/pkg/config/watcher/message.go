package watcher

import (
	"github.com/tumblr/k8s-sidecar-injector/internal/pkg/config"
)

// Message is a message that describes a change and payload to a sidecar configuration
type Message struct {
	Event           Event
	InjectionConfig config.InjectionConfig
}

// Event is what happened to the config (add/delete/update)
type Event uint8

const (
	// EventAdd is a new ConfigMap
	EventAdd Event = iota
	// EventUpdate is an Updated ConfigMap
	EventUpdate
	// EventDelete is a deleted ConfigMap
	EventDelete
)
