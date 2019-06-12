# Design

Just some random notes about the design of the sidecar injector

## Mixin Injections

On the note of why the `k8s-sidecar-injector` does not support multiple injections (aka "mixin" injections), https://github.com/tumblr/k8s-sidecar-injector/issues/17 articulates my thought process during design fairly well. Copying the body of the issue for posterity:

### Q

> Hi, I've been testing your sidecar-injector and just wonder if there is a way to request more than one sidecar injection configurations?
> Multiple annotation "injector.tumblr.com/request" with request assigned to the different names won't raise an error but only the last injection will be applied.

### A

> Great question - this was an explicit design decision I made when writing the injector initially. My thinking was that: while a technically possible and useful feature, it made me nervous about understandabiliity. Supporting "mixin" injections would be neat, but it becomes much more difficult to understand what the injected pod would look like when you need to union multiple injection configs in your head with your pod spec. Further, I was not sure there was an intuitive way to handle merging injection configs that have overlap (i.e. overlapping and conflicting env vars, volumes with diff mount flags, etc).
> Ultimately, I wanted to make sure this was an uninteresting, uncomplicated part of our infrastructure, and not overly clever. We have internal uses that could benefit from multiple injections, but we instead maintain separate config files including the duplication. I have found its easier for new engineers to grok behavior with this design.
