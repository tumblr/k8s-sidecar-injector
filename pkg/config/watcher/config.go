package watcher

// Config is a configuration struct for the Watcher type
type Config struct {
	Namespace       string
	ConfigMapLabels map[string]string
	MasterURL       string
	Kubeconfig      string
}

// NewConfig returns a new initialized Config
func NewConfig() *Config {
	return &Config{
		Namespace:       "",
		ConfigMapLabels: map[string]string{},
		MasterURL:       "",
		Kubeconfig:      "",
	}
}
