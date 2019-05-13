package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/tumblr/k8s-sidecar-injector/internal/pkg/config"
	"github.com/tumblr/k8s-sidecar-injector/internal/pkg/config/watcher"
	"github.com/tumblr/k8s-sidecar-injector/internal/pkg/version"
	"github.com/tumblr/k8s-sidecar-injector/pkg/coalescer"
	"github.com/tumblr/k8s-sidecar-injector/pkg/server"
)

var (
	// EventCoalesceWindow is the window for coalescing events from ConfigMapWatcher
	EventCoalesceWindow = time.Second * 3
)

// ShowVersion shows the version of the jawner
func ShowVersion(o io.Writer) {
	fmt.Fprintf(o, "k8s-sidecar-injector version:%s (commit:%s branch:%s) built on %s with %s\n", version.Version, version.Commit, version.Branch, version.BuildDate, runtime.Version())
}

func init() {
	// set the glog sev to a reasonable default
	flag.Lookup("logtostderr").Value.Set("true")
	// disable logging to disk cause thats strange
	flag.Lookup("log_dir").Value.Set("")
	flag.Lookup("stderrthreshold").Value.Set("INFO")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		ShowVersion(os.Stderr)
		flag.PrintDefaults()
	}
}

func main() {
	var (
		parameters server.Parameters
	)
	cmWatcherLabels := NewMapStringStringFlag()
	watcherConfig := watcher.NewConfig()

	// get command line parameters
	flag.IntVar(&parameters.LifecyclePort, "lifecycle-port", 9000, "Metrics and introspection port (metrics, healthchecking, etc)")
	flag.IntVar(&parameters.TLSPort, "tls-port", 9443, "Webhook server port for handling k8s webhooks (TLS)")
	flag.StringVar(&parameters.CertFile, "tls-cert-file", "/var/lib/secrets/cert.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&parameters.KeyFile, "tls-key-file", "/var/lib/secrets/cert.key", "File containing the x509 private key to --tls-cert-file.")
	flag.StringVar(&parameters.ConfigDirectory, "config-directory", "conf/", "Config directory (will load all .yaml files in this directory)")
	flag.StringVar(&parameters.AnnotationNamespace, "annotation-namespace", "injector.tumblr.com", "Override the AnnotationNamespace")
	flag.StringVar(&watcherConfig.Namespace, "configmap-namespace", "", "Namespace to search for ConfigMaps to load Injection Configs from (default: current namespace)")
	flag.Var(&cmWatcherLabels, "configmap-labels", "Label pairs used to discover ConfigMaps in Kubernetes. These should be key1=value[,key2=val2,...]")
	flag.StringVar(&watcherConfig.MasterURL, "master-url", "", "Kubernetes master URL (used for running outside of the cluster)")
	flag.StringVar(&watcherConfig.Kubeconfig, "kubeconfig", "", "Kubernetes kubeconfig (used only for running outside of the cluster)")
	flag.Parse()

	watcherConfig.ConfigMapLabels = cmWatcherLabels.ToMapStringString()

	glog.Infof("Launching k8s-sidecar-injector version=%s commit=%s branch=%s golang=%s\n", version.Version, version.Commit, version.Branch, runtime.Version())

	glog.V(2).Infof("Loaded server configuration parameters %+v", parameters)
	glog.V(2).Infof("Loaded ConfigMap watcher configuration %+v", watcherConfig)
	cfg, err := config.LoadConfigDirectory(parameters.ConfigDirectory)
	if err != nil {
		glog.Errorf("Failed to load configuration: %v", err)
		os.Exit(1)
	}
	if parameters.AnnotationNamespace != "" {
		cfg.AnnotationNamespace = parameters.AnnotationNamespace
	}

	// wire this up to cancel the context when we get shutdown signal
	ctx, cancelContexts := context.WithCancel(context.Background())

	glog.Infof("Loaded %d injection configs in annotation namespace %s:", len(cfg.Injections), cfg.AnnotationNamespace)
	for _, v := range cfg.Injections {
		glog.Infof("  %s", v.String())
	}

	// start up the watcher, and get the first batch of ConfigMaps
	// to set in the config.
	// make sure to union this with any file configs we loaded from disk
	configWatcher, err := watcher.New(*watcherConfig)
	if err != nil {
		glog.Errorf("Error creating ConfigMap watcher: %s", err.Error())
		os.Exit(1)
	}

	go func() {
		// watch for reconciliation signals, and grab configmaps, then update the running configuration
		// for the server
		sigChan := make(chan interface{}, 10)
		//debouncedChan := make(chan interface{}, 10)

		// debounce events from sigChan, so we dont hammer apiserver on reconciliation
		eventsCh := coalescer.Coalesce(ctx, EventCoalesceWindow, sigChan)

		go func() {
			for {
				glog.Infof("launching watcher for ConfigMaps")
				err := configWatcher.Watch(ctx, sigChan)
				if err != nil {
					switch err {
					case watcher.WatchChannelClosedError:
						glog.Errorf("watcher got error, try to restart watcher: %s", err.Error())
					default:
						glog.Fatalf("error watching for new ConfigMaps (terminating): %s", err.Error())
					}
				}
			}
		}()

		for {
			select {
			case <-eventsCh:
				glog.V(1).Infof("triggering ConfigMap reconciliation")
				updatedInjectionConfigs, err := configWatcher.Get()
				if err != nil {
					glog.Errorf("error reconciling configmaps: %s", err.Error())
					continue
				}
				glog.V(1).Infof("got %d updated InjectionConfigs from reconciliation", len(updatedInjectionConfigs))

				newInjectionConfigs := make([]*config.InjectionConfig, len(updatedInjectionConfigs)+len(cfg.Injections))
				{
					i := 0
					for k := range cfg.Injections {
						newInjectionConfigs[i] = cfg.Injections[k]
						i++
					}
					for i, watched := range updatedInjectionConfigs {
						newInjectionConfigs[i+len(cfg.Injections)] = watched
					}
				}

				glog.V(1).Infof("updating server with newly loaded configurations (%d loaded from disk, %d loaded from k8s api)", len(cfg.Injections), len(updatedInjectionConfigs))
				cfg.ReplaceInjectionConfigs(newInjectionConfigs)
				glog.V(1).Infof("configuration replaced")
			}
		}

	}()

	// web server listening for healthchecks, metrics requests, etc
	lifecycleServer := &http.Server{
		Addr: fmt.Sprintf(":%v", parameters.LifecyclePort),
	}

	// web server terminating TLS for handling k8s webhooks
	whsvr := &server.WebhookServer{
		Config: cfg,
		Server: &http.Server{
			Addr: fmt.Sprintf(":%v", parameters.TLSPort),
		},
	}

	if parameters.CertFile != "" && parameters.KeyFile != "" {
		pair, err := tls.LoadX509KeyPair(parameters.CertFile, parameters.KeyFile)
		if err != nil {
			glog.Errorf("Failed to load key pair: %v", err)
			os.Exit(1)
		}
		whsvr.Server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{pair}}
	}

	// define secure mux for routing requests that come in over our TLS port
	secureMux := mux.NewRouter()
	secureMux.Handle("/mutate", whsvr.MutateHandler())
	secureMux.Handle("/health", whsvr.HealthHandler())
	loggedSecureRouter := handlers.CombinedLoggingHandler(os.Stdout, secureMux)
	whsvr.Server.Handler = loggedSecureRouter

	// start webhook server in new rountine
	glog.Infof("Launching sidecar injector server (http+tls) on :%d", parameters.TLSPort)
	go func() {
		if parameters.CertFile != "" && parameters.KeyFile != "" {
			if err := whsvr.Server.ListenAndServeTLS("", ""); err != nil {
				glog.Errorf("Failed to listen and serve webhook server (http+tls): %v", err)
				os.Exit(1)
			}
		} else {
			if err := whsvr.Server.ListenAndServe(); err != nil {
				glog.Errorf("Failed to listen and serve webhook server (http): %v", err)
				os.Exit(1)
			}
		}
	}()

	// define an insecure mux that handles lifecycle requests
	insecureMux := mux.NewRouter()
	insecureMux.Handle("/metrics", whsvr.MetricsHandler())
	insecureMux.Handle("/health", whsvr.HealthHandler())
	loggedInsecureRouter := handlers.CombinedLoggingHandler(os.Stdout, insecureMux)
	lifecycleServer.Handler = loggedInsecureRouter

	// start webhook server in new rountine
	glog.Infof("Launching lifecycle server (http) on :%d", parameters.LifecyclePort)
	go func() {
		if err := lifecycleServer.ListenAndServe(); err != nil {
			glog.Errorf("Failed to listen and serve lifecycle http server: %v", err)
			os.Exit(1)
		}
	}()

	// listening OS shutdown singal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	glog.Infof("Got OS shutdown signal, shutting down webhook server gracefully...")
	whsvr.Server.Shutdown(ctx)
	cancelContexts()
}
