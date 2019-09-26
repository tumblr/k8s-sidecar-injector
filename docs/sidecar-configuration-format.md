# Sidecar Configuration Format

Config is can be loaded from 2 sources:
* `--config-directory`: load all YAML configs that define a sidecar configuration
* Kubernetes ConfigMaps: `--configmap-labels` and `--configmap-namespace` controls how the injector finds ConfigMaps to load sidecar configurations from

A sidecar configuration looks like:

```yaml
---
# sidecar configs are identified by a requesting
# annotation, like:
# "injector.tumblr.com/request=tumblr-php"
# the "name: tumblr-php" must match a configuration below; 
# Each InjectionConfig is a struct that adheres to kubernetes' volume and containers
# spec. Any volumes injected are scoped to the namespace that the
# resource exists within
name: "tumblr-php"
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
- name: some-config
  configMap:
    name: some-configmap

# hostAliases are not being merged, only added, as they only add entries to /etc/hosts in the containers.
# Duplicate entries won't throw an error.
# hostAliases are used for the whole pod.
hostAliases:
  - ip: 1.2.3.4
    hostnames:
      - somehost.example.com
      - anotherhost.example.com

# all environment variables defined here will be added to containers _only_ if the .Name
# is not already present (we will not replace an env var, only add them)
# These will be inserted into each container in the pod, including any containers added via
# injection. The same applies to volumeMounts.
env:
- name: DATACENTER
  value: "dc01"

# all volumeMounts defined here will be added to containers, if the .name attribute
# does not already exist in the list of volumeMounts, i.e. no replacement will be done.
# They will be added to each container, including the ones added via injection.
# This behaviour is the same for environment variables.
volumeMounts:
  - name: some-config
    mountPath: /etc/some-config

# initContainers will be added, no replacement of existing initContainers with the same names will be done
# this works exactly the same way like adding normal containers does: if you have a conflicting name,
# the server will return an error
initContainers:
  - name: some-initcontainer
    image: init:1.12.2
    imagePullPolicy: IfNotPresent
```

## Configuring new sidecars

In order for the injector to know about a sidecar configuration, you need to either give it a yaml file to describe the sidecar, or create ConfigMaps in Kubernetes (that contain t  he YAML config for the sidecar).

1. Create a new InjectionConfiguration `yaml`
  1. Specify your `name:`. This is what you will request with `injector.tumblr.com/request=$name`
  2. Fill in the `containers`, `volumes`, `volumeMounts`, `hostAliases`, `initContainers` and `env` fields with your configuration you want injected
2. Either bake your yaml into your Docker image you run (in `--config-directory=conf/`), or configure it as a ConfigMap in your k8s cluster. See [/docs/configmaps.md](/docs/configmaps.md) for information on how to configure a ConfigMap.
3. Deploy a pod with annotation `injector.tumblr.com/request=$name`!
