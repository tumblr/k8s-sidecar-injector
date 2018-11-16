package watcher

import (
	"testing"

	_ "github.com/tumblr/k8s-sidecar-injector/internal/pkg/testing"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	testConfig = Config{
		Namespace: "default",
		ConfigMapLabels: map[string]string{
			"thing": "fake",
		},
	}
)

func TestGet(t *testing.T) {
	w := K8sConfigMapWatcher{
		Config: testConfig,
		client: fake.NewSimpleClientset().CoreV1(),
	}

	messages, err := w.Get()
	if err != nil {
		t.Fatal(err.Error())
	}
	if len(messages) != 0 {
		t.Fatalf("expected 0 messages, but got %d", len(messages))
	}
}
