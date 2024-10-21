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

package ssh

import (
	"fmt"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/library/lifecycle"
)

const commandLine = `SSH_ENV_PATH=$HOME/ssh-environment \
&& if [ -f /etc/ssh/passphrase ] && command -v ssh-add >/dev/null; \
then ssh-agent | sed 's/^echo/#echo/' > $SSH_ENV_PATH \
&& chmod 600 $SSH_ENV_PATH && source $SSH_ENV_PATH \
&& ssh-add /etc/ssh/dwo_ssh_key < /etc/ssh/passphrase \
&& if [ -f $HOME/.bashrc ] && [ -w $HOME/.bashrc ]; then echo "source ${SSH_ENV_PATH}" >> $HOME/.bashrc; fi; fi`

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
