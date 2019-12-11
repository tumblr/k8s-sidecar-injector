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

# "name" identifies this sidecar uniquely to the injector. NOTE: it is an error to load
# 2 configuration with the same name! You may include version information in the name to disambiguate
# between newer versions of the same sidecar. For example:
#   name: my-sidecar:v1.2
# indicates "my-sidecar" is version "1.2". A request for `injector.tumblr.com/request: my-sidecar:v1.2`
# will return this configuration. If the version information is omitted, "latest" is assumed.
# `name: "test"` implies `name: test:latest`.
# * `injector.tumblr.com/request: my-sidecar` => `my-sidecar:latest`
# * `injector.tumblr.com/request: my-sidecar:latest` => `my-sidecar:latest`
# * `injector.tumblr.com/request: my-sidecar:v1.2` => `my-sidecar:v1.2`
name: "test:v1.2"

# Each InjectionConfig is a struct that adheres to kubernetes' volume and containers
# spec. Any volumes injected are scoped to the namespace that the
# resource exists within

# Optionally, you can inherit from another sidecar configuration. This is useful to reduce
# duplication in your sidecars. Fields that appear in this config will override and replace
# fields in the inherited sidecar. We intelligently merge list fields as well, so top level
# keys are not blindly replaced, but merged instead.
# `inherits` is a file on disk to load the parent config from.
# NOTE: `inherits` is not supported when loading InjectionConfigs from ConfigMap
# NOTE: this is relative to the current file, and does not allow for absolute pathing!
inherits: "some-sidecar.yaml"

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

# serviceAccountName is optional - if specified, it will set (but not overwrite an existing!)
# serviceAccountName field in your pod. Please note, that due to https://github.com/kubernetes/kubernetes/pull/78080
# if you use this feature on k8s < 1.15.0, your sidecars will not get properly initialized with the associated
# secret volume mounts for this serviceaccount, due to the ServiceAccountController running before
# the MutatingWebhookAdmissionController in older versions of k8s, as well as not _rerunning_ after the MWAC to
# populate volumes on containers that were added by the injector.
serviceAccountName: "someserviceaccount"

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
  2. Fill in the `containers`, `volumes`, `volumeMounts`, `hostAliases`, `initContainers`, `serviceAccountName`, and `env` fields with your configuration you want injected
2. Either bake your yaml into your Docker image you run (in `--config-directory=conf/`), or configure it as a ConfigMap in your k8s cluster. See [/docs/configmaps.md](/docs/configmaps.md) for information on how to configure a ConfigMap.
3. Deploy a pod with annotation `injector.tumblr.com/request=$name`!
