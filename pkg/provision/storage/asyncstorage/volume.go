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

package asyncstorage

import corev1 "k8s.io/api/core/v1"

func GetVolumeFromSecret(secret *corev1.Secret) *corev1.Volume {
	readOnlyPermissions := int32(416) // 0640 (octal) in base-10
	return &corev1.Volume{
		Name: asyncSecretVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secret.Name,
				DefaultMode: &readOnlyPermissions,
			},
		},
	}
}
