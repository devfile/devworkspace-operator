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
	"github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	"github.com/che-incubator/che-workspace-operator/pkg/config"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	oauthv1 "github.com/openshift/api/oauth/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var oauthClientDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(oauthv1.OAuthClient{}, "TypeMeta", "ObjectMeta"),
}

func (r *ReconcileWorkspaceRouting) syncOAuthClient(routing *v1alpha1.WorkspaceRouting, oauthClientSpec oauthv1.OAuthClient) (ok bool, err error) {
	oauthClientInSync := true

	clusterOAuthClients, err := r.getClusterOAuthClients(routing)
	if err != nil {
		return false, err
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

	for _, route := range toDelete {
		err := r.client.Delete(context.TODO(), &route)
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
			err := r.client.Update(context.TODO(), clusterOAuthClient)
			if err != nil && !apierrors.IsConflict(err) {
				return false, err
			}

			oauthClientInSync = false
		}
	} else {
		err = r.client.Create(context.TODO(), &oauthClientSpec)

		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				return true, nil
			}
			return false, err
		}
	}

	return oauthClientInSync, nil
}

func (r *ReconcileWorkspaceRouting) getClusterOAuthClients(routing *v1alpha1.WorkspaceRouting) ([]oauthv1.OAuthClient, error) {
	found := &oauthv1.OAuthClientList{}
	labelSelector, err := labels.Parse(fmt.Sprintf("%s=%s", config.WorkspaceIDLabel, routing.Spec.WorkspaceId))
	if err != nil {
		return nil, err
	}
	listOptions := &client.ListOptions{
		Namespace:     routing.Namespace,
		LabelSelector: labelSelector,
	}
	err = r.client.List(context.TODO(), found, listOptions)
	if err != nil {
		return nil, err
	}

	return found.Items, nil
}

func (r *ReconcileWorkspaceRouting) deleteOAuthClients(routing *v1alpha1.WorkspaceRouting) error {
	labelSelector, err := labels.Parse(fmt.Sprintf("%s=%s", config.WorkspaceIDLabel, routing.Spec.WorkspaceId))
	if err != nil {
		return err
	}
	listOptions := &client.DeleteAllOfOptions{
		ListOptions: *&client.ListOptions{
			Namespace:     routing.Namespace,
			LabelSelector: labelSelector,
		},
	}
	return r.client.DeleteAllOf(context.TODO(), &oauthv1.OAuthClient{}, listOptions)
}
