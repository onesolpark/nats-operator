// Copyright 2017 The nats-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2eutil

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	kubernetesutil "github.com/nats-io/nats-operator/pkg/util/kubernetes"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func DeleteSecrets(kubecli corev1.CoreV1Interface, namespace string, secretNames ...string) error {
	var retErr error
	for _, sname := range secretNames {
		err := kubecli.Secrets(namespace).Delete(sname, metav1.NewDeleteOptions(0))
		if err != nil {
			retErr = fmt.Errorf("failed to delete secret (%s): %v; %v", sname, err, retErr)
		}
	}
	return retErr
}

func KillMembers(kubecli corev1.CoreV1Interface, namespace string, names ...string) error {
	for _, name := range names {
		err := kubecli.Pods(namespace).Delete(name, metav1.NewDeleteOptions(0))
		if err != nil && !kubernetesutil.IsKubernetesResourceNotFoundError(err) {
			return err
		}
	}
	return nil
}

func LogfWithTimestamp(t *testing.T, format string, args ...interface{}) {
	t.Log(time.Now(), fmt.Sprintf(format, args...))
}

func printContainerStatus(buf *bytes.Buffer, ss []v1.ContainerStatus) {
	for _, s := range ss {
		if s.State.Waiting != nil {
			buf.WriteString(fmt.Sprintf("%s: Waiting: message (%s) reason (%s)\n", s.Name, s.State.Waiting.Message, s.State.Waiting.Reason))
		}
		if s.State.Terminated != nil {
			buf.WriteString(fmt.Sprintf("%s: Terminated: message (%s) reason (%s)\n", s.Name, s.State.Terminated.Message, s.State.Terminated.Reason))
		}
	}
}
