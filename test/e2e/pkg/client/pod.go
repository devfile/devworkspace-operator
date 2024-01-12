//
// Copyright (c) 2019-2024 Red Hat, Inc.
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
//

package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (w *K8sClient) WaitForPodRunningByLabel(namespace, label string) (deployed bool, err error) {
	timeout := time.After(6 * time.Minute)
	tick := time.Tick(1 * time.Second)

	for {
		select {
		case <-timeout:
			return false, errors.New("timed out for waiting pod by label " + label)
		case <-tick:
			err = w.WaitForRunningPodBySelector(namespace, label, 3*time.Minute)
			if err == nil {
				return true, nil
			}
		}
	}
}

// Wait up to timeout seconds for all pods in 'namespace' with given 'selector' to enter running state.
// Returns an error if no pods are found or not all discovered pods enter running state.
func (w *K8sClient) WaitForRunningPodBySelector(namespace, selector string, timeout time.Duration) error {
	podList, err := w.ListPods(namespace, selector)
	if err != nil {
		return err
	}
	if len(podList.Items) == 0 {
		log.Printf("Pod not created yet with selector '%s' in namespace %s", selector, namespace)
		return fmt.Errorf("pod not created yet in %s with label %s", namespace, selector)
	}

	for _, pod := range podList.Items {
		log.Printf("Pod '%s' created in namespace %s... Checking startup data.", pod.Name, namespace)
		if err := w.waitForPodRunning(namespace, pod.Name, timeout); err != nil {
			return err
		}
	}

	return nil
}

// Returns the list of currently scheduled or running pods in `namespace` with the given selector
func (w *K8sClient) ListPods(namespace, selector string) (*v1.PodList, error) {
	listOptions := metav1.ListOptions{LabelSelector: selector}
	podList, err := w.Kube().CoreV1().Pods(namespace).List(context.TODO(), listOptions)

	if err != nil {
		return nil, err
	}
	return podList, nil
}

// Poll up to timeout seconds for pod to enter running state.
// Returns an error if the pod never enters the running state.
func (w *K8sClient) waitForPodRunning(namespace, podName string, timeout time.Duration) error {
	return wait.PollImmediate(time.Second, timeout, w.isPodRunning(podName, namespace))
}

// return a condition function that indicates whether the given pod is
// currently running
func (w *K8sClient) isPodRunning(podName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pod, _ := w.Kube().CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		age := time.Since(pod.GetCreationTimestamp().Time).Seconds()

		switch pod.Status.Phase {
		case v1.PodRunning:
			log.Printf("Pod started after %f seconds", age)
			return true, nil
		case v1.PodFailed, v1.PodSucceeded:
			return false, nil
		}
		return false, nil
	}
}

// GetPodNameBySelector returns the pod name that matches selector
// error is returned when
func (w *K8sClient) GetPodNameBySelector(selector, namespace string) (string, error) {
	podList, err := w.ListPods(namespace, selector)

	if err != nil {
		return "", err
	}
	if len(podList.Items) == 0 {
		return "", errors.New(fmt.Sprintf("There is no pod that matches '%s' in namespace %s ", selector, namespace))
	}
	// we expect just 1 pod in test namespace and return the first value from the list
	return podList.Items[0].Name, nil
}
