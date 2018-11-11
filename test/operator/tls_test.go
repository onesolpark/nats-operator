package operatortests

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	k8sv1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8swaitutil "k8s.io/apimachinery/pkg/util/wait"

	"github.com/nats-io/nats-operator/pkg/apis/nats/v1alpha2"
	kubernetesutil "github.com/nats-io/nats-operator/pkg/util/kubernetes"
)

func TestCreateTLSSetup(t *testing.T) {
	t.Skip()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	runController(ctx, t)

	cl, err := newKubeClients()
	if err != nil {
		t.Fatal(err)
	}

	// Wait for the CRDs to become ready.
	if err := kubernetesutil.WaitCRDs(cl.kcrdc); err != nil {
		t.Fatal(err)
	}

	name := "nats"
	namespace := "default"
	var size = 3
	cluster := &v1alpha2.NatsCluster{
		TypeMeta: k8smetav1.TypeMeta{
			Kind:       v1alpha2.CRDResourceKind,
			APIVersion: v1alpha2.SchemeGroupVersion.String(),
		},
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha2.ClusterSpec{
			Size:    size,
			Version: "1.1.0",
			TLS: &v1alpha2.TLSConfig{
				ServerSecret: "nats-certs",
				RoutesSecret: "nats-routes-tls",
			},
		},
	}
	_, err = cl.ncli.Create(ctx, cluster)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = cl.ncli.Delete(ctx, namespace, name)
		if err != nil {
			t.Fatal(err)
		}
	}()

	// Wait for the pods to be created
	params := k8smetav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=nats,nats_cluster=%s", name),
	}
	var podList *k8sv1.PodList
	err = k8swaitutil.Poll(3*time.Second, 1*time.Minute, func() (bool, error) {
		podList, err = cl.kc.Pods(namespace).List(params)
		if err != nil {
			return false, err
		}
		if len(podList.Items) < size {
			return false, nil
		}

		for _, pod := range podList.Items {
			sinceTime := k8smetav1.NewTime(time.Now().Add(time.Duration(-1 * time.Hour)))
			podName := pod.Name
			opts := &k8sv1.PodLogOptions{SinceTime: &sinceTime}
			rc, err := cl.kc.Pods(namespace).GetLogs(podName, opts).Stream()
			if err != nil {
				t.Fatalf("Logs request has failed: %v", err)
			}
			buf := new(bytes.Buffer)
			buf.ReadFrom(rc)
			output := buf.String()
			rc.Close()

			expected := 3
			got := strings.Count(output, "Route connection created")
			if got < expected {
				t.Logf("OUTPUT: %s", output)
				return false, nil
			}
		}

		return true, nil
	})
	if err != nil {
		t.Errorf("Error waiting for pods to be created: %s", err)
	}
	// Give some time for cluster to form
	time.Sleep(2 * time.Second)

	cm, err := cl.kc.Secrets(namespace).Get(name, k8smetav1.GetOptions{})
	if err != nil {
		t.Errorf("Config map error: %v", err)
	}
	conf, ok := cm.Data["nats.conf"]
	if !ok {
		t.Error("Config map was missing")
	}
	for _, pod := range podList.Items {
		if !strings.Contains(string(conf), pod.Name) {
			t.Errorf("Could not find pod %q in config", pod.Name)
		}

		sinceTime := k8smetav1.NewTime(time.Now().Add(time.Duration(-1 * time.Hour)))
		podName := pod.Name
		opts := &k8sv1.PodLogOptions{SinceTime: &sinceTime}
		rc, err := cl.kc.Pods(namespace).GetLogs(podName, opts).Stream()
		if err != nil {
			t.Fatalf("Logs request has failed: %v", err)
		}
		buf := new(bytes.Buffer)
		buf.ReadFrom(rc)

		output := buf.String()

		if !strings.Contains(output, "TLS required for client connections") {
			t.Fatalf("Expected TLS to be required for clients")
		}
		expected := 3
		got := strings.Count(output, "Route connection created")
		if got < expected {
			t.Fatalf("Expected TLS for routes with at least %d connections to be created, got: %d", expected, got)
		}
		rc.Close()
	}
}
