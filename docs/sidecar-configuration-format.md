# Sidecar Configuration Format

Config is can be loaded from 2 sources:
* `--config-directory`: load all YAML configs that define a sidecar configuration
* Kubernetes ConfigMaps: `--configmap-labels` and `--configmap-namespace` controls how the injector finds ConfigMaps to load sidecar configurations from

A sidecar configuration looks like:

```
---
# sidecar configs are identified by a requesting
# annotation, like:
# "injector.tumblr.com/request=tumblr-php"
# the "name: tumblr-php" must match a configuration below; 
# Each InjectionConfig is a struct that adheres to kubernetes' volume and containers
# spec. Any volumes injected are scoped to the namespace that the
# resource exists within
name: "test"
containers:
# we inject a nginx container
- name: sidecar-nginx
  image: nginx:1.12.2
  imagePullPolicy: IfNotPresent
  ports:
    - containerPort: 80
  volumeMounts:
    - name: nginx-conf
      mountPath: /etc/nginx

volumes:
- name: nginx-conf
  configMap:
    name: nginx-configmap

# all environment variables defined here will be added to containers _only_ if the .Name
# is not already present (we will not replace an env var, only add them)
# These will be inserted into each container in the pod, including any containers added via
# injection.
environment:
- name: DATACENTER
  value: "dc01"
```

## Configuring new sidecars

In order for the injector to know about a sidecar configuration, you need to either give it a yaml file to describe the sidecar, or create ConfigMaps in Kubernetes (that contain t  he YAML config for the sidecar).

1. Create a new InjectionConfiguration `yaml`
  1. Specify your `name:`. This is what you will request with `injector.tumblr.com/request=$name`
  2. Fill in the `containers`, `volumes`, and `environment` fields with your configuration you want injected
2. Either bake your yaml into your Docker image you run (in `--config-directory=conf/`), or configure it as a ConfigMap in your k8s cluster. See [/docs/configmaps.md](/docs/configmaps.md) for information on how to configure a ConfigMap.
3. Deploy a pod with annotation `injector.tumblr.com/request=$name`!


