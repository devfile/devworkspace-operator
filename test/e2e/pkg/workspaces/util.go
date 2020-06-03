package workspaces

import (
	"errors"
	"fmt"
	"time"

	"github.com/che-incubator/che-workspace-operator/test/e2e/pkg/metadata"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (w *CodeReady) PodDeployWaitUtil(label string) (deployed bool, err error) {
	timeout := time.After(15 * time.Minute)
	tick := time.Tick(1 * time.Second)

	for {
		select {
		case <-timeout:
			return false, errors.New("timed out")
		case <-tick:
			desc := w.WaitForPodBySelectorRunning(metadata.Namespace.Name, label, 180)
			if desc != nil {
			} else {
				return true, nil
			}
		}
	}
}

// return a condition function that indicates whether the given pod is
// currently running
func (w *CodeReady) isPodRunning(podName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pod, _ := w.Kube().CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
		age := time.Since(pod.GetCreationTimestamp().Time).Seconds()

		switch pod.Status.Phase {
		case v1.PodRunning:
			fmt.Println("Pod started after", age, "seconds")
			return true, nil
		case v1.PodFailed, v1.PodSucceeded:
			return false, nil
		}
		return false, nil
	}
}

// Poll up to timeout seconds for pod to enter running state.
// Returns an error if the pod never enters the running state.
func (w *CodeReady) waitForPodRunning(namespace, podName string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, w.isPodRunning(podName, namespace))
}

// Returns the list of currently scheduled or running pods in `namespace` with the given selector
func (w *CodeReady) ListPods(namespace, selector string) (*v1.PodList, error) {
	listOptions := metav1.ListOptions{LabelSelector: selector}
	podList, err := w.Kube().CoreV1().Pods(namespace).List(listOptions)

	if err != nil {
		return nil, err
	}
	return podList, nil
}

// Wait up to timeout seconds for all pods in 'namespace' with given 'selector' to enter running state.
// Returns an error if no pods are found or not all discovered pods enter running state.
func (w *CodeReady) WaitForPodBySelectorRunning(namespace, selector string, timeout int) error {
	podList, err := w.ListPods(namespace, selector)

	if err != nil {
		return err
	}
	if len(podList.Items) == 0 {
		fmt.Println("Pod not created yet with selector " + selector + " in namespace " + namespace)

		return fmt.Errorf("Pod not created yet in %s with label %s", namespace, selector)
	}

	for _, pod := range podList.Items {
		fmt.Println("Pod " + pod.Name + " created in namespace " + namespace + "...Checking startup data.")
		if err := w.waitForPodRunning(namespace, pod.Name, time.Duration(timeout)*time.Second); err != nil {
			return err
		}
	}

	return nil
}
