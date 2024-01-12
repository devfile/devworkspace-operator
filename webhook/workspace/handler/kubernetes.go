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

package handler

import (
	"context"
	"fmt"
	"strings"

	dwv1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha1"
	dwv2 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"
)

var (
	// Verbs used in SAR checks when users attempt to use a Kubernetes/OpenShift component. A longer list results in
	// more SAR checks per request. We cannot use '*' as a verb as the SAR will check for literal '*' permissions
	// rather than "all verbs"
	userVerbs = []string{"get", "create", "update", "delete"}
)

func (h *WebhookHandler) validateKubernetesObjectPermissionsOnCreate(ctx context.Context, req admission.Request, wksp *dwv2.DevWorkspaceTemplateSpec) error {
	kubeComponents := getKubeComponentsFromWorkspace(wksp)
	for componentName, component := range kubeComponents {
		if !component.GetDeployByDefault() {
			// Intended to be applied later, will not be handled by DWO. It's up to whoever applies it to make
			// sure that's safe to do (e.g. by using the user's token to apply the yaml)
			continue
		}
		if component.Uri != "" {
			// We're going to ignore URI components for now
			continue
		}
		if component.Inlined == "" {
			return fmt.Errorf("kubernetes component does not define inlined content")
		}
		if err := h.validatePermissionsOnObject(ctx, req, componentName, component.Inlined); err != nil {
			return err
		}
	}
	return nil
}

func (h *WebhookHandler) validateKubernetesObjectPermissionsOnUpdate(ctx context.Context, req admission.Request, newWksp, oldWksp *dwv2.DevWorkspaceTemplateSpec) error {
	newKubeComponents := getKubeComponentsFromWorkspace(newWksp)
	oldKubeComponents := getKubeComponentsFromWorkspace(oldWksp)

	for componentName, newComponent := range newKubeComponents {
		if !newComponent.GetDeployByDefault() {
			// Intended to be applied later, will not be handled by DWO. It's up to whoever applies it to make
			// sure that's safe to do (e.g. by using the user's token to apply the yaml)
			continue
		}

		if newComponent.Uri != "" {
			// We're going to ignore URI components for now
			continue
		}
		if newComponent.Inlined == "" {
			return fmt.Errorf("kubernetes component does not define inlined content")
		}

		oldComponent, ok := oldKubeComponents[componentName]
		if !ok || oldComponent.Inlined != newComponent.Inlined {
			// Review new components
			if err := h.validatePermissionsOnObject(ctx, req, componentName, newComponent.Inlined); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *WebhookHandler) validatePermissionsOnObject(ctx context.Context, req admission.Request, componentName, component string) error {

	typeMeta := &metav1.TypeMeta{}
	if err := yaml.Unmarshal([]byte(component), typeMeta); err != nil {
		return fmt.Errorf("failed to read content for component %s", componentName)
	}
	kind := typeMeta.Kind
	if kind == "List" {
		return fmt.Errorf("lists are not supported in Kubernetes or OpenShift components")
	}
	if kind == "Role" || kind == "Rolebinding" || kind == "ClusterRole" || kind == "ClusterRoleBinding" {
		return fmt.Errorf("kubernetes RBAC objects are not permitted within DevWorkspace components")
	}
	if kind == "DevWorkspace" || kind == "DevWorkspaceTemplate" {
		return fmt.Errorf("DevWorkspace objects are not permitted within DevWorkspace components")
	}

	// Workaround to get the correct resource type for a given kind -- probably fragile
	// Convert e.g. Pod -> pods, Deployment -> deployments
	resourceType := fmt.Sprintf("%ss", strings.ToLower(typeMeta.Kind))

	// Check that user has permissions to use the resource
	for _, verb := range userVerbs {
		if err := h.checkSAR(ctx, req, typeMeta, resourceType, verb, componentName); err != nil {
			return err
		}
	}

	// Check that DWO has '*' permissions for the relevant resource
	ssar := &authv1.LocalSubjectAccessReview{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
		},
		Spec: authv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Namespace: req.Namespace,
				Verb:      "*",
				Group:     typeMeta.GroupVersionKind().Group,
				Version:   typeMeta.GroupVersionKind().Version,
				Resource:  resourceType,
			},
			User: h.ControllerSAName,
			UID:  h.ControllerUID,
		},
	}
	if err := h.Client.Create(ctx, ssar); err != nil {
		return fmt.Errorf("failed to create subjectaccessreview for request: %w", err)
	}

	if !ssar.Status.Allowed {
		return fmt.Errorf("devworkspace controller serviceaccount does not have permissions to manage "+
			"kind %s defined in component %s -- an administrator needs to grant the devworkspace operator "+
			"permissions ('*') %s to use this DevWorkspace", kind, componentName, typeMeta.GroupVersionKind().String())
	}

	return nil
}

// getKubeComponentsFromWorkspace returns the Kubernetes (and OpenShift) components in a workspace
// in a map with component names as keys.
func getKubeComponentsFromWorkspace(wksp *dwv2.DevWorkspaceTemplateSpec) map[string]dwv2.K8sLikeComponent {
	kubeComponents := map[string]dwv2.K8sLikeComponent{}
	for _, component := range wksp.Components {
		kubeComponent, err := getKubeLikeComponent(&component)
		if err != nil {
			continue
		}
		kubeComponents[component.Name] = *kubeComponent
	}
	return kubeComponents
}

// getKubeLikeComponent returns the definition of the Kubernetes or OpenShift
// component defined by a general DevWorkspace Component. If the component does
// not specify the Kubernetes or OpenShift field, an error is returned.
func getKubeLikeComponent(component *dwv2.Component) (*dwv2.K8sLikeComponent, error) {
	if component.Kubernetes != nil {
		return &component.Kubernetes.K8sLikeComponent, nil
	}
	if component.Openshift != nil {
		return &component.Openshift.K8sLikeComponent, nil
	}
	return nil, fmt.Errorf("component does not specify kubernetes or openshift fields")
}

func (h *WebhookHandler) validateKubernetesObjectPermissionsOnCreate_v1alpha1(ctx context.Context, req admission.Request, wksp *dwv1.DevWorkspaceTemplateSpec) error {
	kubeComponents := getKubeComponentsFromWorkspace_v1alpha1(wksp)
	for componentName, component := range kubeComponents {
		if component.Uri != "" {
			return fmt.Errorf("kubenetes components specified via URI are unsupported")
		}
		if component.Inlined == "" {
			return fmt.Errorf("kubernetes component does not define inlined content")
		}
		// v1alpha1 DevWorkspace/DevWorkspaceTemplates do not have a deployByDefault field, and the default
		// value in v1alpha2 is false (i.e. do not deploy at start time); however, for safety we check permissions
		// even if the object will not be deployed (v1alpha1 should not be used, in general)
		if err := h.validatePermissionsOnObject(ctx, req, componentName, component.Inlined); err != nil {
			return err
		}
	}
	return nil
}

func (h *WebhookHandler) validateKubernetesObjectPermissionsOnUpdate_v1alpha1(ctx context.Context, req admission.Request, newWksp, oldWksp *dwv1.DevWorkspaceTemplateSpec) error {
	newKubeComponents := getKubeComponentsFromWorkspace_v1alpha1(newWksp)
	oldKubeComponents := getKubeComponentsFromWorkspace_v1alpha1(oldWksp)

	for componentName, newComponent := range newKubeComponents {
		if newComponent.Uri != "" {
			return fmt.Errorf("kubenetes components specified via URI are unsupported")
		}
		if newComponent.Inlined == "" {
			return fmt.Errorf("kubernetes component does not define inlined content")
		}

		// v1alpha1 DevWorkspace/DevWorkspaceTemplates do not have a deployByDefault field, and the default
		// value in v1alpha2 is false (i.e. do not deploy at start time); however, for safety we check permissions
		// even if the object will not be deployed (v1alpha1 should not be used, in general)
		oldComponent, ok := oldKubeComponents[componentName]
		if !ok || oldComponent.Inlined != newComponent.Inlined {
			// Review new components
			if err := h.validatePermissionsOnObject(ctx, req, componentName, newComponent.Inlined); err != nil {
				return err
			}
		}
	}
	return nil
}

func getKubeComponentsFromWorkspace_v1alpha1(wksp *dwv1.DevWorkspaceTemplateSpec) map[string]dwv1.K8sLikeComponent {
	kubeComponents := map[string]dwv1.K8sLikeComponent{}
	for _, component := range wksp.Components {
		kubeComponent, err := getKubeLikeComponent_v1alpha1(&component)
		if err != nil {
			continue
		}
		kubeComponents[kubeComponent.Name] = *kubeComponent
	}
	return kubeComponents
}

func getKubeLikeComponent_v1alpha1(component *dwv1.Component) (*dwv1.K8sLikeComponent, error) {
	if component.Kubernetes != nil {
		return &component.Kubernetes.K8sLikeComponent, nil
	}
	if component.Openshift != nil {
		return &component.Openshift.K8sLikeComponent, nil
	}
	return nil, fmt.Errorf("component does not specify kubernetes or openshift fields")
}

func (h *WebhookHandler) checkSAR(ctx context.Context, req admission.Request, typeMeta *metav1.TypeMeta, resourceType, verb, componentName string) error {
	sar := &authv1.LocalSubjectAccessReview{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
		},
		Spec: authv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Namespace: req.Namespace,
				Verb:      verb,
				Group:     typeMeta.GroupVersionKind().Group,
				Version:   typeMeta.GroupVersionKind().Version,
				Resource:  resourceType,
			},
			User:   req.UserInfo.Username,
			Groups: req.UserInfo.Groups,
			UID:    req.UserInfo.UID,
		},
	}

	err := h.Client.Create(ctx, sar)
	if err != nil {
		return fmt.Errorf("failed to create subjectaccessreview for request: %w", err)
	}

	username := req.UserInfo.Username
	if username == "" {
		username = req.UserInfo.UID
	}
	if !sar.Status.Allowed {
		return fmt.Errorf("user %s does not have permissions to '%s' objects of kind %s defined in component %s", username, verb, typeMeta.GroupVersionKind().String(), componentName)
	}
	return nil
}
