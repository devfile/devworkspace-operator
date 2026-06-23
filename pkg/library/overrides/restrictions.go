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

package overrides

import (
	"strconv"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/common"
	corev1 "k8s.io/api/core/v1"
)

type FieldRestriction struct {
	name              string
	restrictedValue   string
	getRestrictionErr func(msg string) error
}

func (r FieldRestriction) checkAny() error {
	// check any value is restricted
	if r.restrictedValue == "" {
		return r.getRestrictionErr(r.name)
	}

	return nil
}

func (r FieldRestriction) checkString(fieldValue *string) error {
	// value is empty, no restriction applied
	if fieldValue == nil || *fieldValue == "" {
		return nil
	}

	// check any value is restricted
	if r.restrictedValue == "" {
		return r.getRestrictionErr(r.name)
	}

	// check if specific value is restricted
	return r.checkRestrictedValue(*fieldValue)
}

func (r FieldRestriction) checkBool(fieldValue *bool) error {
	if fieldValue == nil {
		return nil
	}

	// check any value is restricted
	if r.restrictedValue == "" {
		return r.getRestrictionErr(r.name)
	}

	// check if specific value is restricted
	return r.checkRestrictedValue(strconv.FormatBool(*fieldValue))
}

func (r FieldRestriction) checkInt32(fieldValue *int32) error {
	if fieldValue == nil {
		return nil
	}

	// check any value is restricted
	if r.restrictedValue == "" {
		return r.getRestrictionErr(r.name)
	}

	// check if specific value is restricted
	return r.checkRestrictedValue(strconv.FormatInt(int64(*fieldValue), 10))
}

func (r FieldRestriction) checkInt64(fieldValue *int64) error {
	if fieldValue == nil {
		return nil
	}

	// check any value is restricted
	if r.restrictedValue == "" {
		return r.getRestrictionErr(r.name)
	}

	return r.checkRestrictedValue(strconv.FormatInt(*fieldValue, 10))
}

func (r FieldRestriction) checkRestrictedValue(value string) error {
	if r.restrictedValue == value {
		return r.getRestrictionErr(r.name + "=" + r.restrictedValue)
	}

	return nil
}

func GetRestrictedContainerOverrideFields(workspace *common.DevWorkspaceWithConfig) []string {
	if workspace.Config != nil && workspace.Config.Workspace != nil && workspace.Config.Workspace.Overrides != nil {
		return workspace.Config.Workspace.Overrides.RestrictedContainerOverrideFields
	}

	return nil
}

func GetRestrictedPodOverrideFields(workspace *common.DevWorkspaceWithConfig) []string {
	if workspace.Config != nil && workspace.Config.Workspace != nil && workspace.Config.Workspace.Overrides != nil {
		return workspace.Config.Workspace.Overrides.RestrictedPodOverrideFields
	}

	return nil
}

func checkResources(resources *corev1.ResourceRequirements, field string, restriction *FieldRestriction) error {
	if resources == nil {
		return nil
	}

	root, remaining, _ := strings.Cut(field, ".")

	if remaining == "" {
		switch root {
		case "limits":
			if resources.Limits != nil {
				return restriction.checkAny()
			}
		case "requests":
			if resources.Requests != nil {
				return restriction.checkAny()
			}
		case "claims":
			if len(resources.Claims) > 0 {
				return restriction.checkAny()
			}
		}
		return nil
	}

	switch root {
	case "limits":
		return checkResourceList(resources.Limits, remaining, restriction)
	case "requests":
		return checkResourceList(resources.Requests, remaining, restriction)
	case "claims":
		return checkContainerResourceClaims(resources.Claims, remaining, restriction)
	}

	return nil
}
func checkResourceList(resources corev1.ResourceList, field string, restriction *FieldRestriction) error {
	if len(resources) == 0 {
		return nil
	}

	for resource := range resources {
		if field == string(resource) {
			if err := restriction.checkAny(); err != nil {
				return err
			}
		}
	}

	return nil
}

func checkContainerResourceClaims(claims []corev1.ResourceClaim, field string, restriction *FieldRestriction) error {
	if len(claims) == 0 {
		return nil
	}

	for _, claim := range claims {
		switch field {
		case "request":
			if err := restriction.checkString(&claim.Request); err != nil {
				return err
			}
		}
	}

	return nil
}
