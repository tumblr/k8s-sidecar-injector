# Runtime Configuration

The sidecar injector has a few needs to get sorted before being able to start injecting sidecars:

1. Generate TLS certs: [/docs/tls.md](/docs/tls.md)
2. `MutatingWebhookConfiguration` resources - this is dependent on the previous step. See [/docs/deployment.md](/docs/deployment.md)
3. Sidecar Configurations - Up to you how you manage these; file or ConfigMap - see [/docs/sidecar-configuration-format.md](/docs/sidecar-configuration-format.md)
4. Deploy to Kubernetes - See [/docs/deployment.md](/docs/deployment.md)

Once you have these sorted out, you should be ready to rock!
