# Generating TLS Certs

Certs are needed to setup the sidecar injector. Because these are not needed to be signed by the k8s apiserver, nor by a valid Certificate Authority, we provide some helpful scripts in `examples/tls/` to create some selfsigned certs for use with this application.

## Edit your CSR configs

To generate new certs for a new deployment, first edit the `ca.conf` and `csr-prod.conf` and tweak them to your liking.

You can use `sed` to do this easily, by setting `DOMAIN=yourdomain.com` and `ORG="Your Org Name, Inc."`:

```bash
$ export ORG="..." DOMAIN="..."
$ sed -i '' -e "s|__ORG__|$ORG|g" -e "s|__DOMAIN__|$DOMAIN|g" ca.conf csr-prod.conf
```

## Generate Certs

Next, set a reasonable value for your deployment's `DEPLOYMENT=` (This is your deployment zone. This can be a physical DC identifier, availability zone, or whatever you term your identifer for geographic blast zones. i.e. `us-east-1` or `dc01`), and your `CLUSTER=` (cluster identifier. $DEPLOYMENT-$CLUSTER uniquely identifies your deployment of k8s). Then, run the script to generate the certs.

Lets take `DEPLOYMENT=us-east-1` and `CLUSTER=PRODUCTION` in our example:

```bash
$ cd examples/tls/
$ DEPLOYMENT=us-east-1 CLUSTER=PRODUCTION ./new-cluster-injector-cert.rb
```

This will generate all the files necessary for a new CA, and the k8s-sidecar-injector cert!

```bash
$ git ls-files -o
.srl
us-east-1/PRODUCTION/ca.crt
us-east-1/PRODUCTION/ca.key
us-east-1/PRODUCTION/sidecar-injector.crt
us-east-1/PRODUCTION/sidecar-injector.csr
us-east-1/PRODUCTION/sidecar-injector.key
```

Now, see the next section to configure the MutatingWebhookConfiguration with the proper certificate :)

# MutatingWebhookConfiguration

The [MutatingWebhookConfiguration](/examples/kubernetes/mutating-webhook-configuration.yaml) needs to know what `ca.crt` is used to sign the certs used to terminate TLS by the service. So, we need to extract the `caBundle` from your generated certificates in the previous step, and set it in [MutatingWebhookConfiguration](/examples/kubernetes/mutating-webhook-configuration.yaml)

Keeping with our `DEPLOYMENT=us-east-1` and `CLUSTER=PRODUCTION` example:

```bash
$ cd examples/tls
$ CABUNDLE_BASE64="$(cat $DEPLOYMENT/$CLUSTER/ca.crt |base64|tr -d '\n')"
$ echo $CABUNDLE_BASE64
LS0tLS1CRUdJTi..........=
```

Now, take this data and set it into the mutating webhook config as the `caBundle:` value.

```bash
$ sed -i '' -e "s|__CA_BUNDLE_BASE64__|$CABUNDLE_BASE64|g" examples/kubernetes/mutating-webhook-configuration.yaml

```

Once this is done, you are ready to head back to [deployment.md](/docs/deployment.md)!

