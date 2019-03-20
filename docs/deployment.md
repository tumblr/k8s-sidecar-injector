# Deployment

Example Kubernetes manifests are provided in [/examples/kubernetes](/examples/kubernetes). You are expected to tailor these to your needs. Specifically, you will need to:

1. Generate TLS certs [/docs/tls.md](/docs/tls.md) and update [/examples/kubernetes/mutating-webhook-configuration.yaml](/examples/kubernetes/mutating-webhook-configuration.yaml) with the `caBundle`
2. Update [/examples/kubernetes/deployment.yaml](/examples/kubernetes/deployment.yaml) with the appropriate version you want to deploy
3. Specify whatever flags you want in the deployment.yaml
4. Create a kubernetes secret from the certificates that you generated as a part of [/docs/tls.md](/docs/tls.md).
```
kubectl create secret generic k8s-sidecar-injector --from-file=examples/tls/${DEPLOYMENT}/${CLUSTER}/sidecar-injector.crt --from-file=examples/tls/${DEPLOYMENT}/${CLUSTER}/sidecar-injector.key --namespace=kube-system
```
5. Create ConfigMaps (or sidecar config files on disk somewhere) so the injector has some sidecars to inject :) [/docs/configmaps.md](/docs/configmaps.md)

Once you hack the example Kubernetes manifests to work for your deployment, deploy them to your cluster. The list of manifests you should deploy are below:

* [clusterrole.yaml](/examples/kubernetes/clusterrole.yaml)
* [clusterrolebinding.yaml](/examples/kubernetes/clusterrolebinding.yaml)
* [service-monitor.yaml](/examples/kubernetes/service-monitor.yaml)
* [serviceaccount.yaml](/examples/kubernetes/serviceaccount.yaml)
* [service.yaml](/examples/kubernetes/service.yaml)
* [deployment.yaml](/examples/kubernetes/deployment.yaml)
* [mutating-webhook-configuration.yaml](/examples/kubernetes/mutating-webhook-configuration.yaml)

A sample ConfigMap is included to test injections at [/examples/kubernetes/configmap-sidecar-test.yaml](/examples/kubernetes/configmap-sidecar-test.yaml).

Add it to the cluster, and you should see it show up in the logs for the sidecar injector.

```bash
$ kubectl create -f examples/kubernetes/configmap-sidecar-test.yaml
configmap/sidecar-test created
$ kubectl logs --tail=60 -n kube-system -l k8s-app=k8s-sidecar-injector
...
I1119 16:25:10.782478       1 main.go:124] triggering ConfigMap reconciliation
I1119 16:25:10.782536       1 watcher.go:140] Fetching ConfigMaps...
I1119 16:25:10.792451       1 watcher.go:147] Fetched 1 ConfigMaps
I1119 16:25:10.792757       1 watcher.go:168] Loaded InjectionConfig test1 from ConfigMap sidecar-test:test1
I1119 16:25:10.792778       1 watcher.go:153] Found 1 InjectionConfigs in sidecar-test
I1119 16:25:10.792788       1 main.go:130] got 1 updated InjectionConfigs from reconciliation
I1119 16:25:10.792800       1 main.go:144] updating server with newly loaded configurations (5 loaded from disk, 1 loaded from k8s api)
I1119 16:25:10.792813       1 main.go:146] configuration replaced
...
```

Now, you are ready to create your first pod that asks for an injection:

```bash
$ kubectl create -f examples/kubernetes/debug-pod.yaml
pod/debian-debug created
```

Verify its up and running; note the `injector.tumblr.com/status: injected` label, indicating the pod had its sidecar added successfully, as well as the added environment variables, and additional `sidecar-nginx` container!

```bash
$ kubectl describe -f debug-pod.yaml
Name:         debian-debug
Namespace:    default
...
Annotations:  injector.tumblr.com/status: injected
Status:       Running
IP:           10.246.248.115
Containers:
  debian-debug:
    Image:         debian:jessie
    Command:
      sleep
      3600
    State:          Running
      Started:      Mon, 19 Nov 2018 11:28:36 -0500
    Ready:          True
    Restart Count:  0
    Environment:
      HELLO:  world
  sidecar-nginx:
    Image:          nginx:1.12.2
    Port:           80/TCP
    Host Port:      0/TCP
    State:          Running
      Started:      Mon, 19 Nov 2018 11:28:40 -0500
    Ready:          True
    Environment:
      HELLO:  world
    Mounts:   <none>
...
```


