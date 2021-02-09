module github.com/tumblr/k8s-sidecar-injector

go 1.15

require (
	github.com/dyson/certman v0.2.1
	github.com/ghodss/yaml v1.0.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/gorilla/handlers v1.4.2
	github.com/gorilla/mux v1.7.4
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/nsf/jsondiff v0.0.0-20200515183724-f29ed568f4ce
	github.com/prometheus/client_golang v1.7.1
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
)
