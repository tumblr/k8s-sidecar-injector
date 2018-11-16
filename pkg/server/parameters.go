package server

// Parameters parameters
type Parameters struct {
	LifecyclePort       int    // metrics, debugging, health checking port (just http)
	TLSPort             int    // webhook server port (forced TLS)
	CertFile            string // path to the x509 certificate for https
	KeyFile             string // path to the x509 private key matching `CertFile`
	ConfigDirectory     string // path to sidecar injector configuration directory (contains yamls)
	AnnotationNamespace string // namespace used to scope annotations
}
