# ConfigMaps

The `k8s-sidecar-injector` is able to read sidecar configuration from kubernetes via `ConfigMap`s, in addition to files on disk. This document describes how the mechanism works.

## Configuration

There are 2 flags that control the injector's behavior regarding loading sidecars from configmaps.

* `--configmap-namespace`: defaults to the namespace the service is running in. This namespace is searched for configmaps to load
* `--configmap-labels=key=value[,key2=value2]`: the labels used to discover configmaps in the API. This is used for a watch, so any new configmaps are loaded when they are created.

These are controlled by `$CONFIGMAP_LABELS` and `$CONFIGMAP_NAMESPACE` in the default entrypoint and deployment.

## ConfigMap Format

A ConfigMap should look like the following; multiple sidecar configs may live in a single ConfigMap:

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-injectionconfig1
  namespace: default
  labels
    app: k8s-sidecar-injector
data:
  sidecar-v1: |
    name: sidecar-v1
    volumes: []
    containers: []
    env: []
  another-sidecar-v1: |
    name: another-sidecar
    env: []
```

Please note, the `labels` must match the provided `$CONFIGMAP_LABELS`, so the injector can discover these ConfigMaps. Additionally, make sure the `namespace` jives with the `$CONFIGMAP_NAMESPACE` (or if omitted, the ConfigMap is in the same namespace as the sidecar injector).

See [/docs/sidecar-configuration-format.md](/docs/sidecar-configuration-format.md) for more details on the schema for a Sidecar Configuration.

## Authentication to read ConfigMaps

The `k8s-sidecar-injector` uses in-cluster discovery of the API, and `ServiceAccount` authentication, which is controlled by the following flags

* `--kubeconfig`: which kubeconfig to use. If omitted, uses in-cluster discovery
* `--master-url`: which kubernetes master to use. If omitted, uses in-cluster discovery

By default, we use `ServiceAccount`s. For this reason, make sure your deployment has
`serviceAccountName: k8s-sidecar-injector`, and you have created the appropriate `ClusterRole`s:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k8s-sidecar-injector
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get","watch","list"]
$ cat clusterrolebinding.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: k8s-sidecar-injector
subjects:
  - kind: ServiceAccount
    name: k8s-sidecar-injector
    namespace: kube-system
roleRef:
  kind: ClusterRole
  name: k8s-sidecar-injector
  apiGroup: rbac.authorization.k8s.io
```

## Watching

This illustrates the injector discovering a new ConfigMap with matching labels, and hot-loading it into the running server:

```
I1113 15:38:41.140753       1 watcher.go:110] event: ADDED &amp;TypeMeta{Kind:,APIVersion:,}
I1113 15:38:41.140809       1 watcher.go:118] signalling event received from watch channel: ADDED &amp;TypeMeta{Kind:,APIVersion:,}
I1113 15:38:41.140837       1 coalescer.go:27] got reconciliation signal, debouncing for 3s
10.246.219.242 - - [13/Nov/2018:15:38:41 +0000] "GET /metrics HTTP/1.1" 200 1768 "" "Prometheus/2.2.1"
10.246.219.236 - - [13/Nov/2018:15:38:43 +0000] "GET /metrics HTTP/1.1" 200 1770 "" "Prometheus/2.2.1"
I1113 15:38:44.140992       1 coalescer.go:21] signalling reconciliation after 3s
I1113 15:38:44.141054       1 main.go:119] triggering ConfigMap reconciliation
I1113 15:38:44.141081       1 watcher.go:141] Fetching ConfigMaps...
I1113 15:38:44.151642       1 watcher.go:148] Fetched 1 ConfigMaps
I1113 15:38:44.151670       1 watcher.go:166] Parsing kube-system/sidecar-test-gabe:test-gabe into InjectionConfig
I1113 15:38:44.151921       1 config.go:139] Loaded injection config env1 sha256sum=a474541e6ea04b5a134f4cf39ee2948484fa9d6c4226514128705d0ba3921c4b
I1113 15:38:44.151951       1 watcher.go:171] Loaded InjectionConfig env1 from ConfigMap sidecar-test-gabe:test-gabe
I1113 15:38:44.151962       1 watcher.go:154] Found 1 InjectionConfigs in sidecar-test-gabe
I1113 15:38:44.151975       1 main.go:125] got 1 updated InjectionConfigs from reconciliation
I1113 15:38:44.151987       1 main.go:139] updating server with newly loaded configurations (5 loaded from disk, 1 loaded from k8s api)
I1113 15:38:44.152004       1 main.go:141] configuration replaced
```


