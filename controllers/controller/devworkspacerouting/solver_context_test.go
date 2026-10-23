// Copyright (c) 2019-2026 Red Hat, Inc.
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

package devworkspacerouting

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
)

func TestReconcilePassesContextToDiscoverableServiceRead(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	scheme := routingTestScheme(t)
	deadline := time.Now().Add(time.Minute)
	routing := &controllerv1alpha1.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{Name: "routing", Namespace: "ns", UID: "routing-uid"},
		Spec: controllerv1alpha1.DevWorkspaceRoutingSpec{
			DevWorkspaceId: "workspace-id",
			RoutingClass:   controllerv1alpha1.DevWorkspaceRoutingCluster,
			Endpoints: map[string]controllerv1alpha1.EndpointList{
				"main": {{
					Name: "api", TargetPort: 8080,
					Exposure:   controllerv1alpha1.InternalEndpointExposure,
					Attributes: controllerv1alpha1.Attributes{}.PutBoolean("discoverable", true),
				}},
			},
		},
		Status: controllerv1alpha1.DevWorkspaceRoutingStatus{
			ObservedGeneration: 4,
			PodAdditions:       &controllerv1alpha1.PodAdditions{Annotations: map[string]string{"existing": "value"}},
			ExposedEndpoints:   map[string]controllerv1alpha1.ExposedEndpointList{"main": {{Name: "existing", Url: "https://example.invalid"}}},
			Phase:              controllerv1alpha1.RoutingPreparing,
			Message:            "before cancellation",
		},
	}
	backing := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(routing).WithObjects(routing).Build()
	clientWithWrites := &serviceWriteRecordingClient{Client: backing}
	ctx, cancel := context.WithDeadline(context.WithValue(context.Background(), testContextKey{}, "reconcile-context"), deadline)
	defer cancel()
	serviceReader := &cancelOnServiceReadClient{Client: backing, cancel: cancel, deadline: deadline}
	r := DevWorkspaceRoutingReconciler{
		Client: clientWithWrites, NonCachingClient: serviceReader, Scheme: scheme,
		Log: logr.Discard(), SolverGetter: &solvers.SolverGetter{},
	}

	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(routing)})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want canceled reconcile context, got %v", err)
	}
	if serviceReader.getCalls != 1 || serviceReader.contextValue != "reconcile-context" || !serviceReader.hasDeadline || !serviceReader.gotDeadline.Equal(deadline) {
		t.Fatalf("discoverable Service read did not receive reconcile context: %#v", serviceReader)
	}
	if clientWithWrites.serviceGets != 0 {
		t.Fatalf("discoverable Service read used cached client %d times", clientWithWrites.serviceGets)
	}
	if clientWithWrites.serviceCreates != 0 || clientWithWrites.serviceUpdates != 0 || clientWithWrites.serviceDeletes != 0 || clientWithWrites.servicePatches != 0 {
		t.Fatalf("canceled reconcile changed Services: creates=%d updates=%d deletes=%d patches=%d", clientWithWrites.serviceCreates, clientWithWrites.serviceUpdates, clientWithWrites.serviceDeletes, clientWithWrites.servicePatches)
	}
	services := &corev1.ServiceList{}
	if err := backing.List(context.Background(), services); err != nil {
		t.Fatal(err)
	}
	if len(services.Items) != 0 {
		t.Fatalf("canceled reconcile created Services: %#v", services.Items)
	}
	actual := &controllerv1alpha1.DevWorkspaceRouting{}
	if err := backing.Get(context.Background(), client.ObjectKeyFromObject(routing), actual); err != nil {
		t.Fatal(err)
	}
	if actual.Status.Phase == controllerv1alpha1.RoutingFailed || !reflect.DeepEqual(actual.Status, routing.Status) {
		t.Fatalf("canceled reconcile changed routing status: want %#v, got %#v", routing.Status, actual.Status)
	}
}

type cancelOnServiceReadClient struct {
	client.Client
	cancel       context.CancelFunc
	deadline     time.Time
	getCalls     int
	contextValue any
	gotDeadline  time.Time
	hasDeadline  bool
}

func (c *cancelOnServiceReadClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*corev1.Service); !ok {
		return c.Client.Get(ctx, key, obj, opts...)
	}
	c.getCalls++
	c.contextValue = ctx.Value(testContextKey{})
	c.gotDeadline, c.hasDeadline = ctx.Deadline()
	if c.contextValue != "reconcile-context" || !c.hasDeadline || !c.gotDeadline.Equal(c.deadline) {
		return errors.New("Service GET did not receive reconcile context")
	}
	c.cancel()
	return ctx.Err()
}

type serviceWriteRecordingClient struct {
	client.Client
	serviceGets    int
	serviceCreates int
	serviceUpdates int
	serviceDeletes int
	servicePatches int
}

func (c *serviceWriteRecordingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*corev1.Service); ok {
		c.serviceGets++
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *serviceWriteRecordingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if _, ok := obj.(*corev1.Service); ok {
		c.serviceCreates++
	}
	return c.Client.Create(ctx, obj, opts...)
}

func (c *serviceWriteRecordingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, ok := obj.(*corev1.Service); ok {
		c.serviceUpdates++
	}
	return c.Client.Update(ctx, obj, opts...)
}

func (c *serviceWriteRecordingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if _, ok := obj.(*corev1.Service); ok {
		c.serviceDeletes++
	}
	return c.Client.Delete(ctx, obj, opts...)
}

func (c *serviceWriteRecordingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if _, ok := obj.(*corev1.Service); ok {
		c.servicePatches++
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}
