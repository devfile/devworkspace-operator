//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package workspacerouting

import (
	"context"
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/config"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	oauthv1 "github.com/openshift/api/oauth/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

var oauthClientDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(oauthv1.OAuthClient{}, "TypeMeta", "ObjectMeta"),
}

func (r *WorkspaceRoutingReconciler) syncOAuthClient(routing *controllerv1alpha1.WorkspaceRouting, oauthClientSpec *oauthv1.OAuthClient) (ok bool, err error) {
	if !config.ControllerCfg.IsOpenShift() {
		return true, nil
	}
	oauthClientInSync := true

	clusterOAuthClients, err := r.getClusterOAuthClients(routing)
	if err != nil {
		return false, err
	}
	if oauthClientSpec == nil {
		if len(clusterOAuthClients) > 0 {
			return false, r.deleteOAuthClients(routing)
		} else {
			return true, nil
		}
	}

	var clusterOAuthClient *oauthv1.OAuthClient
	var toDelete []oauthv1.OAuthClient
	for _, o := range clusterOAuthClients {
		if o.Name == oauthClientSpec.Name {
			clusterOAuthClient = &o
		} else {
			toDelete = append(toDelete, o)
		}
	}

	for _, oauthClient := range toDelete {
		err := r.Delete(context.TODO(), &oauthClient)
		if err != nil {
			return false, err
		}
		oauthClientInSync = false
	}

	if clusterOAuthClient != nil {
		if !cmp.Equal(oauthClientSpec, clusterOAuthClient, oauthClientDiffOpts) {
			// Update oauth client
			clusterOAuthClient.Secret = oauthClientSpec.Secret
			clusterOAuthClient.Labels = oauthClientSpec.Labels
			clusterOAuthClient.GrantMethod = oauthClientSpec.GrantMethod
			clusterOAuthClient.RedirectURIs = oauthClientSpec.RedirectURIs
			err := r.Update(context.TODO(), clusterOAuthClient)
			if err != nil && !apierrors.IsConflict(err) {
				return false, err
			}

			oauthClientInSync = false
		}
	} else {
		err = r.Create(context.TODO(), oauthClientSpec)

		if err != nil && apierrors.IsAlreadyExists(err) {
			return false, nil
		}
		return false, err
	}

	return oauthClientInSync, nil
}

func (r *WorkspaceRoutingReconciler) getClusterOAuthClients(routing *controllerv1alpha1.WorkspaceRouting) ([]oauthv1.OAuthClient, error) {
	found := &oauthv1.OAuthClientList{}
	labelSelector, err := labels.Parse(fmt.Sprintf("%s=%s", config.WorkspaceIDLabel, routing.Spec.WorkspaceId))
	if err != nil {
		return nil, err
	}
	listOptions := &client.ListOptions{
		LabelSelector: labelSelector,
	}
	err = r.List(context.TODO(), found, listOptions)
	if err != nil {
		return nil, err
	}

	return found.Items, nil
}

func (r *WorkspaceRoutingReconciler) deleteOAuthClients(routing *controllerv1alpha1.WorkspaceRouting) error {
	labelSelector, err := labels.Parse(fmt.Sprintf("%s=%s", config.WorkspaceIDLabel, routing.Spec.WorkspaceId))
	if err != nil {
		return err
	}
	listOptions := &client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			LabelSelector: labelSelector,
		},
	}
	return r.DeleteAllOf(context.TODO(), &oauthv1.OAuthClient{}, listOptions)
}
