# Unit Tests

To add a new unit test for some behavior, please do the following:

1. create a AdmissionRequest in YAML format at `test/fixtures/k8s/admissioncontrol/request/foo.yaml`. This should include the pod spec k8s will send with the request, and the annotation with the desired injected sidecar
2. create a Patch JSON at `test/fixtures/k8s/admissioncontrol/patch/foo.json`.
3. register your test in the `pkg/server/webhook_test.go` list `mutationTests`

Please use the `injector.unittest.com/request` annotation on your `AdmissionRequest` YAML to signal which sidecar you want to be injected.
