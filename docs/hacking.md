# Hacking

## Build

Want to build this thing yourself?

```bash
$ make
<magic!>
$ ./bin/k8s-sidecar-injector --help
```

Building the docker image is accomplished by `make docker`

## Tests

```bash
$ make test
```



## Image build

The image is build and published on the Hub at https://hub.docker.com/r/tumblr/k8s-sidecar-injector/. See [/docs/deployment.md](/docs/deployment.md) for how to run this in Kubernetes.

```
$ make docker
```

## Run By Hand

This needs some special configuration surrounding the TLS certs, but if you have already read [docs/configuration.md](./docs/configuration.md), you can run this manually with:

```bash
$ ./bin/k8s-sidecar-injector --tls-port=9000 --config-directory=conf/ --tls-cert-file="${TLS_CERT_FILE}" --tls-key-file="${TLS_KEY_FILE}"
```

NOTE: this is not a supported method of running in production. You are highly encouraged to read [docs/deployment.md](./docs/deployment.md) to deploy this to Kubernetes in The Supported Way.

