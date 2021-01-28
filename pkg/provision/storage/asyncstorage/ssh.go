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

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
)

// GetSSHKeyPair returns a pair of SSH keys. The public key is formatted for inclusion in an
// ssh authorized_keys file, and the private key is pem-formatted.
func GetSSHKeyPair() (public, private []byte, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	pcks := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: pcks,
	}

	privateKeyBytes := pem.EncodeToMemory(privateKeyPEM)

	pubkey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	pubKeyBytes := ssh.MarshalAuthorizedKey(pubkey)

	return pubKeyBytes, privateKeyBytes, nil
}

func ExtractSSHKeyPairFromSecret(secret *corev1.Secret) (public, private []byte, err error) {
	privateKeyPEMBytes, ok := secret.Data[rsyncSSHKeyFilename]
	if !ok {
		return nil, nil, fmt.Errorf("could not find async storage SSH key in secret %s", secret.Name)
	}
	privateKeyPEM, rest := pem.Decode(privateKeyPEMBytes)
	if len(rest) > 0 {
		return nil, nil, fmt.Errorf("additional data encoded in private key")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(privateKeyPEM.Bytes)
	if err != nil {
		return nil, nil, err
	}
	pubkey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	pubkeyBytes := ssh.MarshalAuthorizedKey(pubkey)
	return pubkeyBytes, privateKeyPEMBytes, nil
}
