package watcher

import (
	"context"
	"k8s.io/apimachinery/pkg/watch"
	testcore "k8s.io/client-go/testing"
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

func TestWatcherChannelClose(t *testing.T) {
	client := fake.NewSimpleClientset()
	watcher := watch.NewEmptyWatch()
	client.PrependWatchReactor("configmaps", testcore.DefaultWatchReactor(watcher, nil))

	w := K8sConfigMapWatcher{
		Config: testConfig,
		client: client.CoreV1(),
	}

	sigChan := make(chan interface{}, 10)
	// background context never canceled, no deadline
	ctx := context.Background()

	err := w.Watch(ctx, sigChan)
	if err != nil && err != WatchChannelClosedError {
		t.Errorf("expect catch WatchChannelClosedError, but got %s", err)
	}
}
