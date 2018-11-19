# k8s-sidecar-injector

Uses MutatingAdmissionWebhook in Kubernetes to inject sidecars into new deployments at resource creation time

![Travis (.org)](https://img.shields.io/travis/tumblr/k8s-sidecar-injector.svg)

![Docker Automated build](https://img.shields.io/docker/automated/tumblr/k8s-sidecar-injector.svg)

![Docker Build Status](https://img.shields.io/docker/build/tumblr/k8s-sidecar-injector.svg)

![MicroBadger Size](https://img.shields.io/microbadger/image-size/tumblr/k8s-sidecar-injector.svg)

![Docker Pulls](https://img.shields.io/docker/pulls/tumblr/k8s-sidecar-injector.svg)

![Docker Stars](https://img.shields.io/docker/stars/tumblr/k8s-sidecar-injector.svg)

# What is this?

At Tumblr, we run some containers that have complicated sidecar setups. A kubernetes pod may run 5+ other containers, with some associated volumes and environment variables. It became clear quickly that keeping these sidecars in line would become an operational hassle; making sure every service uses the correct version of each dependency, updating global environment variable sets as configurations in our DCs change, etc. 

To help solve this, we wrote the `k8s-sidecar-injector`. It is a small service that runs in each Kubernetes cluster, and listens to the Kubernetes API via webhooks. For each pod creation, the injector gets a (mutating admission) webhook, asking whether or not to allow the pod launch, and if allowed, what changes we would like to make to it. For pods that have special annotations on them (`injector.tumblr.com/request=some-sidecar-name`), we rewrite the pod configuration to include the containers, volumes, and environment variables defined in the sidecar `some-sidecar-name`'s configuration.

This enabled us to keep sane, centralized configuration for oft-used, but infrequently cared about configuration for our sidecars.

# Configuration

See [/docs/configuration.md](/docs/configuration.md) to get started with setting up your sidecar injector's configurations.

# Deployment

See [/docs/deployment.md](/docs/deployment.md) to see what a sample deployment may look like for you!

# How it works

1. A pod is created. It has annotation `injector.tumblr.com/request=tumblr-php`
2. K8s webhooks out to this service, asking whether to allow this pod creation, and how to mutate it
3a. `If the pod is annotated with `injector.tumblr.com/status=injected`: Do nothing! Return "allowed" to pod creation`
3b. Pull the "tumblr-php" injection config, patch the resource, and return it to k8s
4. Pod will launch in k8s with the modified configuration

# Run

The image is build and published on the Hub at https://hub.docker.com/r/tumblr/k8s-sidecar-injector/. See [/docs/deployment.md](/docs/deployment.md) for how to run this in Kubernetes.

# Hacking

## Build

Want to build this thing yourself? It _should_ be as simple as `make`, but...

```
$ make
<magic!>
$ ./bin/k8s-sidecar-injector --help
```



## By hand

This needs some special configuration surrounding the TLS certs, but if you have already read [docs/configuration.md](./docs/configuration.md), you can run this manually with:

```
$ ./bin/k8s-sidecar-injector --tls-port=9000 --config-directory=conf/ --tls-cert-file="${TLS_CERT_FILE}" --tls-key-file="${TLS_KEY_FILE}"
```

NOTE: this is not a supported method of running in production. You are highly encouraged to read [docs/deployment.md](./docs/deployment.md) to deploy this to Kubernetes in The Supported Way.


# License

[Apache 2.0](/LICENSE.txt)

Copyright 2018, Tumblr, Inc.
