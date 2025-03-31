// Copyright (c) 2019-2025 Red Hat, Inc.
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

package ssh

import (
	"fmt"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/library/lifecycle"
	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const commandLine = `(
SSH_ENV_PATH=$HOME/ssh-environment && \
if [ -f /etc/ssh/passphrase ] && [ -w $HOME ] && command -v ssh-add >/dev/null && command -v ssh-agent >/dev/null; then
    ssh-agent | sed 's/^echo/#echo/' > $SSH_ENV_PATH \
    && chmod 600 $SSH_ENV_PATH \
    && source $SSH_ENV_PATH \
    && if timeout 3 ssh-add /etc/ssh/dwo_ssh_key < /etc/ssh/passphrase && [ -f $HOME/.bashrc ] && [ -w $HOME/.bashrc ]; then
        echo "source ${SSH_ENV_PATH}" >> $HOME/.bashrc
    fi
fi
) || true`

// AddSshAgentPostStartEvent Start ssh-agent and add the default ssh key to it, if the ssh key has a passphrase.
// Initialise the ssh-agent session env variables in the user .bashrc file.
func AddSshAgentPostStartEvent(spec *v1alpha2.DevWorkspaceTemplateSpec) error {
	if spec.Commands == nil {
		spec.Commands = []v1alpha2.Command{}
	}

	if spec.Events == nil {
		spec.Events = &v1alpha2.Events{}
	}

	_, mainComponents, err := lifecycle.GetInitContainers(spec.DevWorkspaceTemplateSpecContent)
	for id, component := range mainComponents {
		if component.Container == nil {
			continue
		}
		commandId := fmt.Sprintf("%s-%d", constants.SshAgentStartEventId, id)
		spec.Commands = append(spec.Commands, v1alpha2.Command{
			Id: commandId,
			CommandUnion: v1alpha2.CommandUnion{
				Exec: &v1alpha2.ExecCommand{
					CommandLine: commandLine,
					Component:   component.Name,
				},
			},
		})
		spec.Events.PostStart = append(spec.Events.PostStart, commandId)
	}
	return err
}

// Determines whether an SSH key with a passphrase is provided for the namespace where the workspace exists.
// If an SSH key with a passphrase is used, then the SSH Agent Post Start event is needed for it to automatically
// be used by the workspace's SSH agent.
// If no SSH key is provided, or the SSH key does not provide a passphrase, then the SSH Agent Post Start event is not required.
func NeedsSSHPostStartEvent(api sync.ClusterAPI, namespace string) (bool, error) {
	secretNN := types.NamespacedName{
		Name:      constants.SSHSecretName,
		Namespace: namespace,
	}
	sshSecret := &corev1.Secret{}
	if err := api.Client.Get(api.Ctx, secretNN, sshSecret); err != nil {
		if k8sErrors.IsNotFound(err) {
			// No SSH secret found
			return false, nil
		}
		return false, err
	}

	if _, ok := sshSecret.Data[constants.SSHSecretPassphraseKey]; ok {
		// SSH secret exists and has a passphrase set
		return true, nil
	}

	return false, nil
}
