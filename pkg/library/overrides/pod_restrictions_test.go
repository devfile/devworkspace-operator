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
	"testing"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/ptr"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func TestRestrictPodOverride(t *testing.T) {
	tests := []struct {
		Name             string
		RestrictedFields []string
		Override         corev1.PodSpec
		IsErrorExpected  bool
		ErrField         string
	}{
		{
			Name:     "no denied fields allows everything",
			Override: corev1.PodSpec{},
		},
		{
			Name:            "containers always denied",
			Override:        corev1.PodSpec{Containers: []corev1.Container{{}}},
			IsErrorExpected: true,
			ErrField:        "containers",
		},
		{
			Name:            "initContainers always denied",
			Override:        corev1.PodSpec{InitContainers: []corev1.Container{{}}},
			IsErrorExpected: true,
			ErrField:        "initContainers",
		},
		// ----------- RestartPolicy -----------
		{
			Name:             "restartPolicy empty value not restricted",
			Override:         corev1.PodSpec{RestartPolicy: ""},
			RestrictedFields: []string{"restartPolicy"},
			IsErrorExpected:  false,
		},
		{
			Name:             "restartPolicy restricted by specific value match",
			Override:         corev1.PodSpec{RestartPolicy: corev1.RestartPolicyAlways},
			RestrictedFields: []string{"restartPolicy=Always"},
			IsErrorExpected:  true,
			ErrField:         "restartPolicy=Always",
		},
		{
			Name:             "restartPolicy restricted by any value",
			Override:         corev1.PodSpec{RestartPolicy: corev1.RestartPolicyAlways},
			RestrictedFields: []string{"restartPolicy"},
			IsErrorExpected:  true,
			ErrField:         "restartPolicy",
		},
		{
			Name:             "restartPolicy allowed when restricted value does not match",
			Override:         corev1.PodSpec{RestartPolicy: corev1.RestartPolicyAlways},
			RestrictedFields: []string{"restartPolicy=Never"},
			IsErrorExpected:  false,
		},
		// ----------- TerminationGracePeriodSeconds -----------
		{
			Name:             "terminationGracePeriodSeconds nil value not restricted",
			Override:         corev1.PodSpec{TerminationGracePeriodSeconds: nil},
			RestrictedFields: []string{"terminationGracePeriodSeconds"},
			IsErrorExpected:  false,
		},
		{
			Name:             "terminationGracePeriodSeconds restricted by specific value match",
			Override:         corev1.PodSpec{TerminationGracePeriodSeconds: ptr.To(int64(30))},
			RestrictedFields: []string{"terminationGracePeriodSeconds=30"},
			IsErrorExpected:  true,
			ErrField:         "terminationGracePeriodSeconds=30",
		},
		{
			Name:             "terminationGracePeriodSeconds restricted by any value",
			Override:         corev1.PodSpec{TerminationGracePeriodSeconds: ptr.To(int64(30))},
			RestrictedFields: []string{"terminationGracePeriodSeconds"},
			IsErrorExpected:  true,
			ErrField:         "terminationGracePeriodSeconds",
		},
		{
			Name:             "terminationGracePeriodSeconds allowed when restricted value does not match",
			Override:         corev1.PodSpec{TerminationGracePeriodSeconds: ptr.To(int64(30))},
			RestrictedFields: []string{"terminationGracePeriodSeconds=60"},
			IsErrorExpected:  false,
		},
		// ----------- ActiveDeadlineSeconds -----------
		{
			Name:             "activeDeadlineSeconds nil value not restricted",
			Override:         corev1.PodSpec{ActiveDeadlineSeconds: nil},
			RestrictedFields: []string{"activeDeadlineSeconds"},
			IsErrorExpected:  false,
		},
		{
			Name:             "activeDeadlineSeconds restricted by specific value match",
			Override:         corev1.PodSpec{ActiveDeadlineSeconds: ptr.To(int64(600))},
			RestrictedFields: []string{"activeDeadlineSeconds=600"},
			IsErrorExpected:  true,
			ErrField:         "activeDeadlineSeconds=600",
		},
		{
			Name:             "activeDeadlineSeconds restricted by any value",
			Override:         corev1.PodSpec{ActiveDeadlineSeconds: ptr.To(int64(600))},
			RestrictedFields: []string{"activeDeadlineSeconds"},
			IsErrorExpected:  true,
			ErrField:         "activeDeadlineSeconds",
		},
		{
			Name:             "activeDeadlineSeconds allowed when restricted value does not match",
			Override:         corev1.PodSpec{ActiveDeadlineSeconds: ptr.To(int64(600))},
			RestrictedFields: []string{"activeDeadlineSeconds=300"},
			IsErrorExpected:  false,
		},
		// ----------- DNSPolicy -----------
		{
			Name:             "dnsPolicy empty value not restricted",
			Override:         corev1.PodSpec{DNSPolicy: ""},
			RestrictedFields: []string{"dnsPolicy"},
			IsErrorExpected:  false,
		},
		{
			Name:             "dnsPolicy restricted by specific value match",
			Override:         corev1.PodSpec{DNSPolicy: corev1.DNSClusterFirst},
			RestrictedFields: []string{"dnsPolicy=ClusterFirst"},
			IsErrorExpected:  true,
			ErrField:         "dnsPolicy=ClusterFirst",
		},
		{
			Name:             "dnsPolicy restricted by any value",
			Override:         corev1.PodSpec{DNSPolicy: corev1.DNSClusterFirst},
			RestrictedFields: []string{"dnsPolicy"},
			IsErrorExpected:  true,
			ErrField:         "dnsPolicy",
		},
		{
			Name:             "dnsPolicy allowed when restricted value does not match",
			Override:         corev1.PodSpec{DNSPolicy: corev1.DNSClusterFirst},
			RestrictedFields: []string{"dnsPolicy=Default"},
			IsErrorExpected:  false,
		},
		// ----------- NodeSelector -----------
		{
			Name:             "nodeSelector empty map not restricted",
			Override:         corev1.PodSpec{NodeSelector: map[string]string{}},
			RestrictedFields: []string{"nodeSelector"},
			IsErrorExpected:  false,
		},
		{
			Name:             "nodeSelector restricted by any value",
			Override:         corev1.PodSpec{NodeSelector: map[string]string{"disktype": "ssd"}},
			RestrictedFields: []string{"nodeSelector"},
			IsErrorExpected:  true,
			ErrField:         "nodeSelector",
		},
		// ----------- ServiceAccountName -----------
		{
			Name:             "serviceAccountName empty value not restricted",
			Override:         corev1.PodSpec{ServiceAccountName: ""},
			RestrictedFields: []string{"serviceAccountName"},
			IsErrorExpected:  false,
		},
		{
			Name:             "serviceAccountName restricted by specific value match",
			Override:         corev1.PodSpec{ServiceAccountName: "my-sa"},
			RestrictedFields: []string{"serviceAccountName=my-sa"},
			IsErrorExpected:  true,
			ErrField:         "serviceAccountName=my-sa",
		},
		{
			Name:             "serviceAccountName restricted by any value",
			Override:         corev1.PodSpec{ServiceAccountName: "my-sa"},
			RestrictedFields: []string{"serviceAccountName"},
			IsErrorExpected:  true,
			ErrField:         "serviceAccountName",
		},
		{
			Name:             "serviceAccountName allowed when restricted value does not match",
			Override:         corev1.PodSpec{ServiceAccountName: "my-sa"},
			RestrictedFields: []string{"serviceAccountName=other-sa"},
			IsErrorExpected:  false,
		},
		// ----------- DeprecatedServiceAccount -----------
		{
			Name:             "deprecatedServiceAccount empty value not restricted",
			Override:         corev1.PodSpec{DeprecatedServiceAccount: ""},
			RestrictedFields: []string{"deprecatedServiceAccount"},
			IsErrorExpected:  false,
		},
		{
			Name:             "deprecatedServiceAccount restricted by specific value match",
			Override:         corev1.PodSpec{DeprecatedServiceAccount: "my-sa"},
			RestrictedFields: []string{"deprecatedServiceAccount=my-sa"},
			IsErrorExpected:  true,
			ErrField:         "deprecatedServiceAccount=my-sa",
		},
		{
			Name:             "deprecatedServiceAccount restricted by any value",
			Override:         corev1.PodSpec{DeprecatedServiceAccount: "my-sa"},
			RestrictedFields: []string{"deprecatedServiceAccount"},
			IsErrorExpected:  true,
			ErrField:         "deprecatedServiceAccount",
		},
		{
			Name:             "deprecatedServiceAccount allowed when restricted value does not match",
			Override:         corev1.PodSpec{DeprecatedServiceAccount: "my-sa"},
			RestrictedFields: []string{"deprecatedServiceAccount=other-sa"},
			IsErrorExpected:  false,
		},
		// ----------- AutomountServiceAccountToken -----------
		{
			Name:             "automountServiceAccountToken nil value not restricted",
			Override:         corev1.PodSpec{AutomountServiceAccountToken: nil},
			RestrictedFields: []string{"automountServiceAccountToken"},
			IsErrorExpected:  false,
		},
		{
			Name:             "automountServiceAccountToken restricted by specific value match",
			Override:         corev1.PodSpec{AutomountServiceAccountToken: ptr.To(true)},
			RestrictedFields: []string{"automountServiceAccountToken=true"},
			IsErrorExpected:  true,
			ErrField:         "automountServiceAccountToken=true",
		},
		{
			Name:             "automountServiceAccountToken restricted by any value",
			Override:         corev1.PodSpec{AutomountServiceAccountToken: ptr.To(true)},
			RestrictedFields: []string{"automountServiceAccountToken"},
			IsErrorExpected:  true,
			ErrField:         "automountServiceAccountToken",
		},
		{
			Name:             "automountServiceAccountToken allowed when restricted value does not match",
			Override:         corev1.PodSpec{AutomountServiceAccountToken: ptr.To(true)},
			RestrictedFields: []string{"automountServiceAccountToken=false"},
			IsErrorExpected:  false,
		},
		// ----------- NodeName -----------
		{
			Name:             "nodeName empty value not restricted",
			Override:         corev1.PodSpec{NodeName: ""},
			RestrictedFields: []string{"nodeName"},
			IsErrorExpected:  false,
		},
		{
			Name:             "nodeName restricted by specific value match",
			Override:         corev1.PodSpec{NodeName: "node-1"},
			RestrictedFields: []string{"nodeName=node-1"},
			IsErrorExpected:  true,
			ErrField:         "nodeName=node-1",
		},
		{
			Name:             "nodeName restricted by any value",
			Override:         corev1.PodSpec{NodeName: "node-1"},
			RestrictedFields: []string{"nodeName"},
			IsErrorExpected:  true,
			ErrField:         "nodeName",
		},
		{
			Name:             "nodeName allowed when restricted value does not match",
			Override:         corev1.PodSpec{NodeName: "node-1"},
			RestrictedFields: []string{"nodeName=node-2"},
			IsErrorExpected:  false,
		},
		// ----------- HostIPC -----------
		{
			Name:             "hostIPC false value restricted by any value",
			Override:         corev1.PodSpec{HostIPC: false},
			RestrictedFields: []string{"hostIPC"},
			IsErrorExpected:  true,
			ErrField:         "hostIPC",
		},
		{
			Name:             "hostIPC restricted by specific value match",
			Override:         corev1.PodSpec{HostIPC: true},
			RestrictedFields: []string{"hostIPC=true"},
			IsErrorExpected:  true,
			ErrField:         "hostIPC=true",
		},
		{
			Name:             "hostIPC restricted by any value",
			Override:         corev1.PodSpec{HostIPC: true},
			RestrictedFields: []string{"hostIPC"},
			IsErrorExpected:  true,
			ErrField:         "hostIPC",
		},
		{
			Name:             "hostIPC allowed when restricted value does not match",
			Override:         corev1.PodSpec{HostIPC: true},
			RestrictedFields: []string{"hostIPC=false"},
			IsErrorExpected:  false,
		},
		// ----------- HostPID -----------
		{
			Name:             "hostPID false value restricted by any value",
			Override:         corev1.PodSpec{HostPID: false},
			RestrictedFields: []string{"hostPID"},
			IsErrorExpected:  true,
			ErrField:         "hostPID",
		},
		{
			Name:             "hostPID restricted by specific value match",
			Override:         corev1.PodSpec{HostPID: true},
			RestrictedFields: []string{"hostPID=true"},
			IsErrorExpected:  true,
			ErrField:         "hostPID=true",
		},
		{
			Name:             "hostPID restricted by any value",
			Override:         corev1.PodSpec{HostPID: true},
			RestrictedFields: []string{"hostPID"},
			IsErrorExpected:  true,
			ErrField:         "hostPID",
		},
		{
			Name:             "hostPID allowed when restricted value does not match",
			Override:         corev1.PodSpec{HostPID: true},
			RestrictedFields: []string{"hostPID=false"},
			IsErrorExpected:  false,
		},
		// ----------- HostNetwork -----------
		{
			Name:             "hostNetwork false value restricted by any value",
			Override:         corev1.PodSpec{HostNetwork: false},
			RestrictedFields: []string{"hostNetwork"},
			IsErrorExpected:  true,
			ErrField:         "hostNetwork",
		},
		{
			Name:             "hostNetwork restricted by specific value match",
			Override:         corev1.PodSpec{HostNetwork: true},
			RestrictedFields: []string{"hostNetwork=true"},
			IsErrorExpected:  true,
			ErrField:         "hostNetwork=true",
		},
		{
			Name:             "hostNetwork restricted by any value",
			Override:         corev1.PodSpec{HostNetwork: true},
			RestrictedFields: []string{"hostNetwork"},
			IsErrorExpected:  true,
			ErrField:         "hostNetwork",
		},
		{
			Name:             "hostNetwork allowed when restricted value does not match",
			Override:         corev1.PodSpec{HostNetwork: true},
			RestrictedFields: []string{"hostNetwork=false"},
			IsErrorExpected:  false,
		},
		// ----------- ShareProcessNamespace -----------
		{
			Name:             "shareProcessNamespace nil value not restricted",
			Override:         corev1.PodSpec{ShareProcessNamespace: nil},
			RestrictedFields: []string{"shareProcessNamespace"},
			IsErrorExpected:  false,
		},
		{
			Name:             "shareProcessNamespace restricted by specific value match",
			Override:         corev1.PodSpec{ShareProcessNamespace: ptr.To(true)},
			RestrictedFields: []string{"shareProcessNamespace=true"},
			IsErrorExpected:  true,
			ErrField:         "shareProcessNamespace=true",
		},
		{
			Name:             "shareProcessNamespace restricted by any value",
			Override:         corev1.PodSpec{ShareProcessNamespace: ptr.To(true)},
			RestrictedFields: []string{"shareProcessNamespace"},
			IsErrorExpected:  true,
			ErrField:         "shareProcessNamespace",
		},
		{
			Name:             "shareProcessNamespace allowed when restricted value does not match",
			Override:         corev1.PodSpec{ShareProcessNamespace: ptr.To(true)},
			RestrictedFields: []string{"shareProcessNamespace=false"},
			IsErrorExpected:  false,
		},
		// ----------- Hostname -----------
		{
			Name:             "hostname empty value not restricted",
			Override:         corev1.PodSpec{Hostname: ""},
			RestrictedFields: []string{"hostname"},
			IsErrorExpected:  false,
		},
		{
			Name:             "hostname restricted by specific value match",
			Override:         corev1.PodSpec{Hostname: "my-host"},
			RestrictedFields: []string{"hostname=my-host"},
			IsErrorExpected:  true,
			ErrField:         "hostname=my-host",
		},
		{
			Name:             "hostname restricted by any value",
			Override:         corev1.PodSpec{Hostname: "my-host"},
			RestrictedFields: []string{"hostname"},
			IsErrorExpected:  true,
			ErrField:         "hostname",
		},
		{
			Name:             "hostname allowed when restricted value does not match",
			Override:         corev1.PodSpec{Hostname: "my-host"},
			RestrictedFields: []string{"hostname=other-host"},
			IsErrorExpected:  false,
		},
		// ----------- Subdomain -----------
		{
			Name:             "subdomain empty value not restricted",
			Override:         corev1.PodSpec{Subdomain: ""},
			RestrictedFields: []string{"subdomain"},
			IsErrorExpected:  false,
		},
		{
			Name:             "subdomain restricted by specific value match",
			Override:         corev1.PodSpec{Subdomain: "my-subdomain"},
			RestrictedFields: []string{"subdomain=my-subdomain"},
			IsErrorExpected:  true,
			ErrField:         "subdomain=my-subdomain",
		},
		{
			Name:             "subdomain restricted by any value",
			Override:         corev1.PodSpec{Subdomain: "my-subdomain"},
			RestrictedFields: []string{"subdomain"},
			IsErrorExpected:  true,
			ErrField:         "subdomain",
		},
		{
			Name:             "subdomain allowed when restricted value does not match",
			Override:         corev1.PodSpec{Subdomain: "my-subdomain"},
			RestrictedFields: []string{"subdomain=other"},
			IsErrorExpected:  false,
		},
		// ----------- SchedulerName -----------
		{
			Name:             "schedulerName empty value not restricted",
			Override:         corev1.PodSpec{SchedulerName: ""},
			RestrictedFields: []string{"schedulerName"},
			IsErrorExpected:  false,
		},
		{
			Name:             "schedulerName restricted by specific value match",
			Override:         corev1.PodSpec{SchedulerName: "custom-scheduler"},
			RestrictedFields: []string{"schedulerName=custom-scheduler"},
			IsErrorExpected:  true,
			ErrField:         "schedulerName=custom-scheduler",
		},
		{
			Name:             "schedulerName restricted by any value",
			Override:         corev1.PodSpec{SchedulerName: "custom-scheduler"},
			RestrictedFields: []string{"schedulerName"},
			IsErrorExpected:  true,
			ErrField:         "schedulerName",
		},
		{
			Name:             "schedulerName allowed when restricted value does not match",
			Override:         corev1.PodSpec{SchedulerName: "custom-scheduler"},
			RestrictedFields: []string{"schedulerName=default-scheduler"},
			IsErrorExpected:  false,
		},
		// ----------- PriorityClassName -----------
		{
			Name:             "priorityClassName empty value not restricted",
			Override:         corev1.PodSpec{PriorityClassName: ""},
			RestrictedFields: []string{"priorityClassName"},
			IsErrorExpected:  false,
		},
		{
			Name:             "priorityClassName restricted by specific value match",
			Override:         corev1.PodSpec{PriorityClassName: "high-priority"},
			RestrictedFields: []string{"priorityClassName=high-priority"},
			IsErrorExpected:  true,
			ErrField:         "priorityClassName=high-priority",
		},
		{
			Name:             "priorityClassName restricted by any value",
			Override:         corev1.PodSpec{PriorityClassName: "high-priority"},
			RestrictedFields: []string{"priorityClassName"},
			IsErrorExpected:  true,
			ErrField:         "priorityClassName",
		},
		{
			Name:             "priorityClassName allowed when restricted value does not match",
			Override:         corev1.PodSpec{PriorityClassName: "high-priority"},
			RestrictedFields: []string{"priorityClassName=low-priority"},
			IsErrorExpected:  false,
		},
		// ----------- Priority -----------
		{
			Name:             "priority nil value not restricted",
			Override:         corev1.PodSpec{Priority: nil},
			RestrictedFields: []string{"priority"},
			IsErrorExpected:  false,
		},
		{
			Name:             "priority restricted by specific value match",
			Override:         corev1.PodSpec{Priority: ptr.To(int32(1000))},
			RestrictedFields: []string{"priority=1000"},
			IsErrorExpected:  true,
			ErrField:         "priority=1000",
		},
		{
			Name:             "priority restricted by any value",
			Override:         corev1.PodSpec{Priority: ptr.To(int32(1000))},
			RestrictedFields: []string{"priority"},
			IsErrorExpected:  true,
			ErrField:         "priority",
		},
		{
			Name:             "priority allowed when restricted value does not match",
			Override:         corev1.PodSpec{Priority: ptr.To(int32(1000))},
			RestrictedFields: []string{"priority=2000"},
			IsErrorExpected:  false,
		},
		// ----------- RuntimeClassName -----------
		{
			Name:             "runtimeClassName nil value not restricted",
			Override:         corev1.PodSpec{RuntimeClassName: nil},
			RestrictedFields: []string{"runtimeClassName"},
			IsErrorExpected:  false,
		},
		{
			Name:             "runtimeClassName restricted by specific value match",
			Override:         corev1.PodSpec{RuntimeClassName: ptr.To("kata")},
			RestrictedFields: []string{"runtimeClassName=kata"},
			IsErrorExpected:  true,
			ErrField:         "runtimeClassName=kata",
		},
		{
			Name:             "runtimeClassName restricted by any value",
			Override:         corev1.PodSpec{RuntimeClassName: ptr.To("kata")},
			RestrictedFields: []string{"runtimeClassName"},
			IsErrorExpected:  true,
			ErrField:         "runtimeClassName",
		},
		{
			Name:             "runtimeClassName allowed when restricted value does not match",
			Override:         corev1.PodSpec{RuntimeClassName: ptr.To("kata")},
			RestrictedFields: []string{"runtimeClassName=gvisor"},
			IsErrorExpected:  false,
		},
		// ----------- EnableServiceLinks -----------
		{
			Name:             "enableServiceLinks nil value not restricted",
			Override:         corev1.PodSpec{EnableServiceLinks: nil},
			RestrictedFields: []string{"enableServiceLinks"},
			IsErrorExpected:  false,
		},
		{
			Name:             "enableServiceLinks restricted by specific value match",
			Override:         corev1.PodSpec{EnableServiceLinks: ptr.To(true)},
			RestrictedFields: []string{"enableServiceLinks=true"},
			IsErrorExpected:  true,
			ErrField:         "enableServiceLinks=true",
		},
		{
			Name:             "enableServiceLinks restricted by any value",
			Override:         corev1.PodSpec{EnableServiceLinks: ptr.To(true)},
			RestrictedFields: []string{"enableServiceLinks"},
			IsErrorExpected:  true,
			ErrField:         "enableServiceLinks",
		},
		{
			Name:             "enableServiceLinks allowed when restricted value does not match",
			Override:         corev1.PodSpec{EnableServiceLinks: ptr.To(true)},
			RestrictedFields: []string{"enableServiceLinks=false"},
			IsErrorExpected:  false,
		},
		// ----------- PreemptionPolicy -----------
		{
			Name:             "preemptionPolicy nil value not restricted",
			Override:         corev1.PodSpec{PreemptionPolicy: nil},
			RestrictedFields: []string{"preemptionPolicy"},
			IsErrorExpected:  false,
		},
		{
			Name:             "preemptionPolicy restricted by specific value match",
			Override:         corev1.PodSpec{PreemptionPolicy: preemptionPolicyPtr(corev1.PreemptNever)},
			RestrictedFields: []string{"preemptionPolicy=Never"},
			IsErrorExpected:  true,
			ErrField:         "preemptionPolicy=Never",
		},
		{
			Name:             "preemptionPolicy restricted by any value",
			Override:         corev1.PodSpec{PreemptionPolicy: preemptionPolicyPtr(corev1.PreemptNever)},
			RestrictedFields: []string{"preemptionPolicy"},
			IsErrorExpected:  true,
			ErrField:         "preemptionPolicy",
		},
		{
			Name:             "preemptionPolicy allowed when restricted value does not match",
			Override:         corev1.PodSpec{PreemptionPolicy: preemptionPolicyPtr(corev1.PreemptNever)},
			RestrictedFields: []string{"preemptionPolicy=PreemptLowerPriority"},
			IsErrorExpected:  false,
		},
		// ----------- SetHostnameAsFQDN -----------
		{
			Name:             "setHostnameAsFQDN nil value not restricted",
			Override:         corev1.PodSpec{SetHostnameAsFQDN: nil},
			RestrictedFields: []string{"setHostnameAsFQDN"},
			IsErrorExpected:  false,
		},
		{
			Name:             "setHostnameAsFQDN restricted by specific value match",
			Override:         corev1.PodSpec{SetHostnameAsFQDN: ptr.To(true)},
			RestrictedFields: []string{"setHostnameAsFQDN=true"},
			IsErrorExpected:  true,
			ErrField:         "setHostnameAsFQDN=true",
		},
		{
			Name:             "setHostnameAsFQDN restricted by any value",
			Override:         corev1.PodSpec{SetHostnameAsFQDN: ptr.To(true)},
			RestrictedFields: []string{"setHostnameAsFQDN"},
			IsErrorExpected:  true,
			ErrField:         "setHostnameAsFQDN",
		},
		{
			Name:             "setHostnameAsFQDN allowed when restricted value does not match",
			Override:         corev1.PodSpec{SetHostnameAsFQDN: ptr.To(true)},
			RestrictedFields: []string{"setHostnameAsFQDN=false"},
			IsErrorExpected:  false,
		},
		// ----------- HostUsers -----------
		{
			Name:             "hostUsers nil value not restricted",
			Override:         corev1.PodSpec{HostUsers: nil},
			RestrictedFields: []string{"hostUsers"},
			IsErrorExpected:  false,
		},
		{
			Name:             "hostUsers restricted by specific value match",
			Override:         corev1.PodSpec{HostUsers: ptr.To(true)},
			RestrictedFields: []string{"hostUsers=true"},
			IsErrorExpected:  true,
			ErrField:         "hostUsers=true",
		},
		{
			Name:             "hostUsers restricted by any value",
			Override:         corev1.PodSpec{HostUsers: ptr.To(true)},
			RestrictedFields: []string{"hostUsers"},
			IsErrorExpected:  true,
			ErrField:         "hostUsers",
		},
		{
			Name:             "hostUsers allowed when restricted value does not match",
			Override:         corev1.PodSpec{HostUsers: ptr.To(true)},
			RestrictedFields: []string{"hostUsers=false"},
			IsErrorExpected:  false,
		},
		// ----------- HostnameOverride -----------
		{
			Name:             "hostnameOverride nil value not restricted",
			Override:         corev1.PodSpec{HostnameOverride: nil},
			RestrictedFields: []string{"hostnameOverride"},
			IsErrorExpected:  false,
		},
		{
			Name:             "hostnameOverride restricted by specific value match",
			Override:         corev1.PodSpec{HostnameOverride: ptr.To("custom-hostname")},
			RestrictedFields: []string{"hostnameOverride=custom-hostname"},
			IsErrorExpected:  true,
			ErrField:         "hostnameOverride=custom-hostname",
		},
		{
			Name:             "hostnameOverride restricted by any value",
			Override:         corev1.PodSpec{HostnameOverride: ptr.To("custom-hostname")},
			RestrictedFields: []string{"hostnameOverride"},
			IsErrorExpected:  true,
			ErrField:         "hostnameOverride",
		},
		{
			Name:             "hostnameOverride allowed when restricted value does not match",
			Override:         corev1.PodSpec{HostnameOverride: ptr.To("custom-hostname")},
			RestrictedFields: []string{"hostnameOverride=other-hostname"},
			IsErrorExpected:  false,
		},
		// ----------- EphemeralContainers -----------
		{
			Name:             "ephemeralContainers empty slice not restricted",
			Override:         corev1.PodSpec{EphemeralContainers: nil},
			RestrictedFields: []string{"ephemeralContainers"},
			IsErrorExpected:  false,
		},
		{
			Name:             "ephemeralContainers restricted by any value",
			Override:         corev1.PodSpec{EphemeralContainers: []corev1.EphemeralContainer{{}}},
			RestrictedFields: []string{"ephemeralContainers"},
			IsErrorExpected:  true,
			ErrField:         "ephemeralContainers",
		},
		// ----------- Affinity -----------
		{
			Name:             "affinity nil not restricted",
			Override:         corev1.PodSpec{Affinity: nil},
			RestrictedFields: []string{"affinity"},
			IsErrorExpected:  false,
		},
		{
			Name:             "affinity restricted by any value",
			Override:         corev1.PodSpec{Affinity: &corev1.Affinity{}},
			RestrictedFields: []string{"affinity"},
			IsErrorExpected:  true,
			ErrField:         "affinity",
		},
		// ----------- Tolerations -----------
		{
			Name:             "tolerations empty slice not restricted",
			Override:         corev1.PodSpec{Tolerations: nil},
			RestrictedFields: []string{"tolerations"},
			IsErrorExpected:  false,
		},
		{
			Name:             "tolerations restricted by any value",
			Override:         corev1.PodSpec{Tolerations: []corev1.Toleration{{}}},
			RestrictedFields: []string{"tolerations"},
			IsErrorExpected:  true,
			ErrField:         "tolerations",
		},
		// ----------- HostAliases -----------
		{
			Name:             "hostAliases empty slice not restricted",
			Override:         corev1.PodSpec{HostAliases: nil},
			RestrictedFields: []string{"hostAliases"},
			IsErrorExpected:  false,
		},
		{
			Name:             "hostAliases restricted by any value",
			Override:         corev1.PodSpec{HostAliases: []corev1.HostAlias{{}}},
			RestrictedFields: []string{"hostAliases"},
			IsErrorExpected:  true,
			ErrField:         "hostAliases",
		},
		// ----------- DNSConfig -----------
		{
			Name:             "dnsConfig nil not restricted",
			Override:         corev1.PodSpec{DNSConfig: nil},
			RestrictedFields: []string{"dnsConfig"},
			IsErrorExpected:  false,
		},
		{
			Name:             "dnsConfig restricted by any value",
			Override:         corev1.PodSpec{DNSConfig: &corev1.PodDNSConfig{}},
			RestrictedFields: []string{"dnsConfig"},
			IsErrorExpected:  true,
			ErrField:         "dnsConfig",
		},
		// ----------- ReadinessGates -----------
		{
			Name:             "readinessGates empty slice not restricted",
			Override:         corev1.PodSpec{ReadinessGates: nil},
			RestrictedFields: []string{"readinessGates"},
			IsErrorExpected:  false,
		},
		{
			Name:             "readinessGates restricted by any value",
			Override:         corev1.PodSpec{ReadinessGates: []corev1.PodReadinessGate{{}}},
			RestrictedFields: []string{"readinessGates"},
			IsErrorExpected:  true,
			ErrField:         "readinessGates",
		},
		// ----------- TopologySpreadConstraints -----------
		{
			Name:             "topologySpreadConstraints empty slice not restricted",
			Override:         corev1.PodSpec{TopologySpreadConstraints: nil},
			RestrictedFields: []string{"topologySpreadConstraints"},
			IsErrorExpected:  false,
		},
		{
			Name:             "topologySpreadConstraints restricted by any value",
			Override:         corev1.PodSpec{TopologySpreadConstraints: []corev1.TopologySpreadConstraint{{}}},
			RestrictedFields: []string{"topologySpreadConstraints"},
			IsErrorExpected:  true,
			ErrField:         "topologySpreadConstraints",
		},
		// ----------- OS -----------
		{
			Name:             "os nil not restricted",
			Override:         corev1.PodSpec{OS: nil},
			RestrictedFields: []string{"os"},
			IsErrorExpected:  false,
		},
		{
			Name:             "os restricted by any value",
			Override:         corev1.PodSpec{OS: &corev1.PodOS{}},
			RestrictedFields: []string{"os"},
			IsErrorExpected:  true,
			ErrField:         "os",
		},
		// ----------- WorkloadRef -----------
		{
			Name:             "schedulingGates empty slice not restricted",
			Override:         corev1.PodSpec{WorkloadRef: nil},
			RestrictedFields: []string{"workloadRef"},
			IsErrorExpected:  false,
		},
		{
			Name:             "schedulingGates restricted by any value",
			Override:         corev1.PodSpec{WorkloadRef: &corev1.WorkloadReference{}},
			RestrictedFields: []string{"workloadRef"},
			IsErrorExpected:  true,
			ErrField:         "workloadRef",
		},
		// ----------- SchedulingGates -----------
		{
			Name:             "schedulingGates empty slice not restricted",
			Override:         corev1.PodSpec{SchedulingGates: nil},
			RestrictedFields: []string{"schedulingGates"},
			IsErrorExpected:  false,
		},
		{
			Name:             "schedulingGates restricted by any value",
			Override:         corev1.PodSpec{SchedulingGates: []corev1.PodSchedulingGate{{}}},
			RestrictedFields: []string{"schedulingGates"},
			IsErrorExpected:  true,
			ErrField:         "schedulingGates",
		},
		// ----------- Volumes -----------
		{
			Name:             "volumes empty slice not restricted",
			Override:         corev1.PodSpec{Volumes: nil},
			RestrictedFields: []string{"volumes"},
			IsErrorExpected:  false,
		},
		{
			Name:             "volumes restricted by any value",
			Override:         corev1.PodSpec{Volumes: []corev1.Volume{{}}},
			RestrictedFields: []string{"volumes"},
			IsErrorExpected:  true,
			ErrField:         "volumes",
		},
		// ----------- Volumes.Name -----------
		{
			Name:             "volumes.name empty value not restricted",
			Override:         corev1.PodSpec{Volumes: []corev1.Volume{{Name: ""}}},
			RestrictedFields: []string{"volumes.name"},
			IsErrorExpected:  false,
		},
		{
			Name:             "volumes.name restricted by specific value match",
			Override:         corev1.PodSpec{Volumes: []corev1.Volume{{Name: "my-volume"}}},
			RestrictedFields: []string{"volumes.name=my-volume"},
			IsErrorExpected:  true,
			ErrField:         "volumes.name=my-volume",
		},
		{
			Name:             "volumes.name restricted by any value",
			Override:         corev1.PodSpec{Volumes: []corev1.Volume{{Name: "my-volume"}}},
			RestrictedFields: []string{"volumes.name"},
			IsErrorExpected:  true,
			ErrField:         "volumes.name",
		},
		{
			Name:             "volumes.name allowed when restricted value does not match",
			Override:         corev1.PodSpec{Volumes: []corev1.Volume{{Name: "my-volume"}}},
			RestrictedFields: []string{"volumes.name=other-volume"},
			IsErrorExpected:  false,
		},
		// ----------- Volumes source type -----------
		{
			Name:             "volumes.hostPath restricted by any value",
			Override:         corev1.PodSpec{Volumes: []corev1.Volume{{Name: "vol", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/host"}}}}},
			RestrictedFields: []string{"volumes.hostPath"},
			IsErrorExpected:  true,
			ErrField:         "volumes.hostPath",
		},
		{
			Name:             "volumes.emptyDir not restricted when restriction is for different type",
			Override:         corev1.PodSpec{Volumes: []corev1.Volume{{Name: "vol", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}},
			RestrictedFields: []string{"volumes.hostPath"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext -----------
		{
			Name:             "securityContext nil not restricted",
			Override:         corev1.PodSpec{SecurityContext: nil},
			RestrictedFields: []string{"securityContext"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext restricted by any value",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{}},
			RestrictedFields: []string{"securityContext"},
			IsErrorExpected:  true,
			ErrField:         "securityContext",
		},
		// ----------- SecurityContext.SELinuxOptions -----------
		{
			Name:             "securityContext.seLinuxOptions nil not restricted",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SELinuxOptions: nil}},
			RestrictedFields: []string{"securityContext.seLinuxOptions"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.seLinuxOptions restricted by any value",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SELinuxOptions: &corev1.SELinuxOptions{}}},
			RestrictedFields: []string{"securityContext.seLinuxOptions"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.seLinuxOptions",
		},
		// ----------- SecurityContext.RunAsUser -----------
		{
			Name:             "securityContext.runAsUser nil value not restricted",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{RunAsUser: nil}},
			RestrictedFields: []string{"securityContext.runAsUser"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.runAsUser restricted by specific value match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{RunAsUser: ptr.To(int64(1000))}},
			RestrictedFields: []string{"securityContext.runAsUser=1000"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.runAsUser=1000",
		},
		{
			Name:             "securityContext.runAsUser restricted by any value",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{RunAsUser: ptr.To(int64(1000))}},
			RestrictedFields: []string{"securityContext.runAsUser"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.runAsUser",
		},
		{
			Name:             "securityContext.runAsUser allowed when restricted value does not match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{RunAsUser: ptr.To(int64(1000))}},
			RestrictedFields: []string{"securityContext.runAsUser=2000"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.RunAsGroup -----------
		{
			Name:             "securityContext.runAsGroup nil value not restricted",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{RunAsGroup: nil}},
			RestrictedFields: []string{"securityContext.runAsGroup"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.runAsGroup restricted by specific value match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{RunAsGroup: ptr.To(int64(500))}},
			RestrictedFields: []string{"securityContext.runAsGroup=500"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.runAsGroup=500",
		},
		{
			Name:             "securityContext.runAsGroup restricted by any value",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{RunAsGroup: ptr.To(int64(500))}},
			RestrictedFields: []string{"securityContext.runAsGroup"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.runAsGroup",
		},
		{
			Name:             "securityContext.runAsGroup allowed when restricted value does not match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{RunAsGroup: ptr.To(int64(500))}},
			RestrictedFields: []string{"securityContext.runAsGroup=1000"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.RunAsNonRoot -----------
		{
			Name:             "securityContext.runAsNonRoot nil value not restricted",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: nil}},
			RestrictedFields: []string{"securityContext.runAsNonRoot"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.runAsNonRoot restricted by specific value match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: ptr.To(true)}},
			RestrictedFields: []string{"securityContext.runAsNonRoot=true"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.runAsNonRoot=true",
		},
		{
			Name:             "securityContext.runAsNonRoot restricted by any value",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: ptr.To(true)}},
			RestrictedFields: []string{"securityContext.runAsNonRoot"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.runAsNonRoot",
		},
		{
			Name:             "securityContext.runAsNonRoot allowed when restricted value does not match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: ptr.To(true)}},
			RestrictedFields: []string{"securityContext.runAsNonRoot=false"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.SupplementalGroups -----------
		{
			Name:             "securityContext.supplementalGroups empty slice not restricted",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SupplementalGroups: nil}},
			RestrictedFields: []string{"securityContext.supplementalGroups"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.supplementalGroups restricted by any value",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SupplementalGroups: []int64{1000}}},
			RestrictedFields: []string{"securityContext.supplementalGroups"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.supplementalGroups",
		},
		{
			Name:             "securityContext.supplementalGroups restricted by specific value match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SupplementalGroups: []int64{1000}}},
			RestrictedFields: []string{"securityContext.supplementalGroups=1000"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.supplementalGroups=1000",
		},
		{
			Name:             "securityContext.supplementalGroups allowed when restricted value does not match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SupplementalGroups: []int64{1000}}},
			RestrictedFields: []string{"securityContext.supplementalGroups=500"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.SupplementalGroupsPolicy -----------
		{
			Name:             "securityContext.supplementalGroupsPolicy nil value not restricted",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SupplementalGroupsPolicy: nil}},
			RestrictedFields: []string{"securityContext.supplementalGroupsPolicy"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.supplementalGroupsPolicy restricted by specific value match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SupplementalGroupsPolicy: supplementalGroupsPolicyPtr(corev1.SupplementalGroupsPolicyMerge)}},
			RestrictedFields: []string{"securityContext.supplementalGroupsPolicy=Merge"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.supplementalGroupsPolicy=Merge",
		},
		{
			Name:             "securityContext.supplementalGroupsPolicy restricted by any value",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SupplementalGroupsPolicy: supplementalGroupsPolicyPtr(corev1.SupplementalGroupsPolicyMerge)}},
			RestrictedFields: []string{"securityContext.supplementalGroupsPolicy"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.supplementalGroupsPolicy",
		},
		{
			Name:             "securityContext.supplementalGroupsPolicy allowed when restricted value does not match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SupplementalGroupsPolicy: supplementalGroupsPolicyPtr(corev1.SupplementalGroupsPolicyMerge)}},
			RestrictedFields: []string{"securityContext.supplementalGroupsPolicy=Strict"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.FSGroup -----------
		{
			Name:             "securityContext.fsGroup nil value not restricted",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{FSGroup: nil}},
			RestrictedFields: []string{"securityContext.fsGroup"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.fsGroup restricted by specific value match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{FSGroup: ptr.To(int64(1000))}},
			RestrictedFields: []string{"securityContext.fsGroup=1000"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.fsGroup=1000",
		},
		{
			Name:             "securityContext.fsGroup restricted by any value",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{FSGroup: ptr.To(int64(1000))}},
			RestrictedFields: []string{"securityContext.fsGroup"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.fsGroup",
		},
		{
			Name:             "securityContext.fsGroup allowed when restricted value does not match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{FSGroup: ptr.To(int64(1000))}},
			RestrictedFields: []string{"securityContext.fsGroup=2000"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.Sysctls -----------
		{
			Name:             "securityContext.sysctls empty slice not restricted",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{Sysctls: nil}},
			RestrictedFields: []string{"securityContext.sysctls"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.sysctls restricted by any value",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{Sysctls: []corev1.Sysctl{{Name: "net.core.somaxconn", Value: "1024"}}}},
			RestrictedFields: []string{"securityContext.sysctls"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.sysctls",
		},
		{
			Name:             "securityContext.sysctls restricted by specific value match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{Sysctls: []corev1.Sysctl{{Name: "net.core.somaxconn", Value: "1024"}}}},
			RestrictedFields: []string{"securityContext.sysctls.name=net.core.somaxconn"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.sysctls.name=net.core.somaxconn",
		},
		{
			Name:             "securityContext.sysctls allowed when restricted value does not match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{Sysctls: []corev1.Sysctl{{Name: "net.core.somaxconn", Value: "1024"}}}},
			RestrictedFields: []string{"securityContext.sysctls.name=kernel.shm_rmid_forced"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.FSGroupChangePolicy -----------
		{
			Name:             "securityContext.fsGroupChangePolicy nil value not restricted",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{FSGroupChangePolicy: nil}},
			RestrictedFields: []string{"securityContext.fsGroupChangePolicy"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.fsGroupChangePolicy restricted by specific value match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{FSGroupChangePolicy: fsGroupChangePolicyPtr(corev1.FSGroupChangeOnRootMismatch)}},
			RestrictedFields: []string{"securityContext.fsGroupChangePolicy=OnRootMismatch"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.fsGroupChangePolicy=OnRootMismatch",
		},
		{
			Name:             "securityContext.fsGroupChangePolicy restricted by any value",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{FSGroupChangePolicy: fsGroupChangePolicyPtr(corev1.FSGroupChangeOnRootMismatch)}},
			RestrictedFields: []string{"securityContext.fsGroupChangePolicy"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.fsGroupChangePolicy",
		},
		{
			Name:             "securityContext.fsGroupChangePolicy allowed when restricted value does not match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{FSGroupChangePolicy: fsGroupChangePolicyPtr(corev1.FSGroupChangeOnRootMismatch)}},
			RestrictedFields: []string{"securityContext.fsGroupChangePolicy=Always"},
			IsErrorExpected:  false,
		},
		// ----------- SecurityContext.SeccompProfile -----------
		{
			Name:             "securityContext.seccompProfile nil not restricted",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SeccompProfile: nil}},
			RestrictedFields: []string{"securityContext.seccompProfile"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.seccompProfile restricted by any value",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SeccompProfile: &corev1.SeccompProfile{}}},
			RestrictedFields: []string{"securityContext.seccompProfile"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.seccompProfile",
		},
		// ----------- SecurityContext.AppArmorProfile -----------
		{
			Name:             "securityContext.appArmorProfile nil not restricted",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{AppArmorProfile: nil}},
			RestrictedFields: []string{"securityContext.appArmorProfile"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.appArmorProfile restricted by any value",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{AppArmorProfile: &corev1.AppArmorProfile{}}},
			RestrictedFields: []string{"securityContext.appArmorProfile"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.appArmorProfile",
		},
		// ----------- SecurityContext.SELinuxChangePolicy -----------
		{
			Name:             "securityContext.seLinuxChangePolicy nil value not restricted",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SELinuxChangePolicy: nil}},
			RestrictedFields: []string{"securityContext.seLinuxChangePolicy"},
			IsErrorExpected:  false,
		},
		{
			Name:             "securityContext.seLinuxChangePolicy restricted by specific value match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SELinuxChangePolicy: seLinuxChangePolicyPtr(corev1.SELinuxChangePolicyRecursive)}},
			RestrictedFields: []string{"securityContext.seLinuxChangePolicy=Recursive"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.seLinuxChangePolicy=Recursive",
		},
		{
			Name:             "securityContext.seLinuxChangePolicy restricted by any value",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SELinuxChangePolicy: seLinuxChangePolicyPtr(corev1.SELinuxChangePolicyRecursive)}},
			RestrictedFields: []string{"securityContext.seLinuxChangePolicy"},
			IsErrorExpected:  true,
			ErrField:         "securityContext.seLinuxChangePolicy",
		},
		{
			Name:             "securityContext.seLinuxChangePolicy allowed when restricted value does not match",
			Override:         corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{SELinuxChangePolicy: seLinuxChangePolicyPtr(corev1.SELinuxChangePolicyRecursive)}},
			RestrictedFields: []string{"securityContext.seLinuxChangePolicy=MountOption"},
			IsErrorExpected:  false,
		},
		// ----------- ImagePullSecrets -----------
		{
			Name:             "imagePullSecrets empty slice not restricted",
			Override:         corev1.PodSpec{ImagePullSecrets: nil},
			RestrictedFields: []string{"imagePullSecrets"},
			IsErrorExpected:  false,
		},
		{
			Name:             "imagePullSecrets restricted by any value",
			Override:         corev1.PodSpec{ImagePullSecrets: []corev1.LocalObjectReference{{}}},
			RestrictedFields: []string{"imagePullSecrets"},
			IsErrorExpected:  true,
			ErrField:         "imagePullSecrets",
		},
		// ----------- ImagePullSecrets.Name -----------
		{
			Name:             "imagePullSecrets.name empty value not restricted",
			Override:         corev1.PodSpec{ImagePullSecrets: []corev1.LocalObjectReference{{Name: ""}}},
			RestrictedFields: []string{"imagePullSecrets.name"},
			IsErrorExpected:  false,
		},
		{
			Name:             "imagePullSecrets.name restricted by specific value match",
			Override:         corev1.PodSpec{ImagePullSecrets: []corev1.LocalObjectReference{{Name: "my-secret"}}},
			RestrictedFields: []string{"imagePullSecrets.name=my-secret"},
			IsErrorExpected:  true,
			ErrField:         "imagePullSecrets.name=my-secret",
		},
		{
			Name:             "imagePullSecrets.name restricted by any value",
			Override:         corev1.PodSpec{ImagePullSecrets: []corev1.LocalObjectReference{{Name: "my-secret"}}},
			RestrictedFields: []string{"imagePullSecrets.name"},
			IsErrorExpected:  true,
			ErrField:         "imagePullSecrets.name",
		},
		{
			Name:             "imagePullSecrets.name allowed when restricted value does not match",
			Override:         corev1.PodSpec{ImagePullSecrets: []corev1.LocalObjectReference{{Name: "my-secret"}}},
			RestrictedFields: []string{"imagePullSecrets.name=other-secret"},
			IsErrorExpected:  false,
		},
		// ----------- Overhead -----------
		{
			Name:             "overhead empty not restricted",
			Override:         corev1.PodSpec{Overhead: nil},
			RestrictedFields: []string{"overhead"},
			IsErrorExpected:  false,
		},
		{
			Name:             "overhead restricted by any value",
			Override:         corev1.PodSpec{Overhead: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}},
			RestrictedFields: []string{"overhead"},
			IsErrorExpected:  true,
			ErrField:         "overhead",
		},
		// ----------- Overhead.CPU -----------
		{
			Name:             "overhead.cpu restricted by any value",
			Override:         corev1.PodSpec{Overhead: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}},
			RestrictedFields: []string{"overhead.cpu"},
			IsErrorExpected:  true,
			ErrField:         "overhead.cpu",
		},
		// ----------- Overhead.Memory -----------
		{
			Name:             "overhead.memory restricted by any value",
			Override:         corev1.PodSpec{Overhead: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("128Mi")}},
			RestrictedFields: []string{"overhead.memory"},
			IsErrorExpected:  true,
			ErrField:         "overhead.memory",
		},
		// ----------- ResourceClaims -----------
		{
			Name:             "resourceClaims empty slice not restricted",
			Override:         corev1.PodSpec{ResourceClaims: nil},
			RestrictedFields: []string{"resourceClaims"},
			IsErrorExpected:  false,
		},
		{
			Name:             "resourceClaims restricted by any value",
			Override:         corev1.PodSpec{ResourceClaims: []corev1.PodResourceClaim{{}}},
			RestrictedFields: []string{"resourceClaims"},
			IsErrorExpected:  true,
			ErrField:         "resourceClaims",
		},
		// ----------- ResourceClaims.Name -----------
		{
			Name:             "resourceClaims.name empty slice not restricted",
			Override:         corev1.PodSpec{ResourceClaims: nil},
			RestrictedFields: []string{"resourceClaims.name"},
			IsErrorExpected:  false,
		},
		{
			Name:             "resourceClaims.name restricted by specific value match",
			Override:         corev1.PodSpec{ResourceClaims: []corev1.PodResourceClaim{{Name: "gpu"}}},
			RestrictedFields: []string{"resourceClaims.name=gpu"},
			IsErrorExpected:  true,
			ErrField:         "resourceClaims.name=gpu",
		},
		{
			Name:             "resourceClaims.name restricted by any value",
			Override:         corev1.PodSpec{ResourceClaims: []corev1.PodResourceClaim{{Name: "gpu"}}},
			RestrictedFields: []string{"resourceClaims.name"},
			IsErrorExpected:  true,
			ErrField:         "resourceClaims.name",
		},
		{
			Name:             "resourceClaims.name allowed when restricted value does not match",
			Override:         corev1.PodSpec{ResourceClaims: []corev1.PodResourceClaim{{Name: "gpu"}}},
			RestrictedFields: []string{"resourceClaims.name=tpu"},
			IsErrorExpected:  false,
		},
		// ----------- ResourceClaims.ResourceClaimName -----------
		{
			Name:             "resourceClaims.resourceClaimName nil value not restricted",
			Override:         corev1.PodSpec{ResourceClaims: []corev1.PodResourceClaim{{Name: "gpu", ResourceClaimName: nil}}},
			RestrictedFields: []string{"resourceClaims.resourceClaimName"},
			IsErrorExpected:  false,
		},
		{
			Name:             "resourceClaims.resourceClaimName restricted by specific value match",
			Override:         corev1.PodSpec{ResourceClaims: []corev1.PodResourceClaim{{Name: "gpu", ResourceClaimName: ptr.To("my-claim")}}},
			RestrictedFields: []string{"resourceClaims.resourceClaimName=my-claim"},
			IsErrorExpected:  true,
			ErrField:         "resourceClaims.resourceClaimName=my-claim",
		},
		{
			Name:             "resourceClaims.resourceClaimName restricted by any value",
			Override:         corev1.PodSpec{ResourceClaims: []corev1.PodResourceClaim{{Name: "gpu", ResourceClaimName: ptr.To("my-claim")}}},
			RestrictedFields: []string{"resourceClaims.resourceClaimName"},
			IsErrorExpected:  true,
			ErrField:         "resourceClaims.resourceClaimName",
		},
		{
			Name:             "resourceClaims.resourceClaimName allowed when restricted value does not match",
			Override:         corev1.PodSpec{ResourceClaims: []corev1.PodResourceClaim{{Name: "gpu", ResourceClaimName: ptr.To("my-claim")}}},
			RestrictedFields: []string{"resourceClaims.resourceClaimName=other-claim"},
			IsErrorExpected:  false,
		},
		// ----------- ResourceClaims.ResourceClaimTemplateName -----------
		{
			Name:             "resourceClaims.resourceClaimTemplateName nil value not restricted",
			Override:         corev1.PodSpec{ResourceClaims: []corev1.PodResourceClaim{{Name: "gpu", ResourceClaimTemplateName: nil}}},
			RestrictedFields: []string{"resourceClaims.resourceClaimTemplateName"},
			IsErrorExpected:  false,
		},
		{
			Name:             "resourceClaims.resourceClaimTemplateName restricted by specific value match",
			Override:         corev1.PodSpec{ResourceClaims: []corev1.PodResourceClaim{{Name: "gpu", ResourceClaimTemplateName: ptr.To("my-template")}}},
			RestrictedFields: []string{"resourceClaims.resourceClaimTemplateName=my-template"},
			IsErrorExpected:  true,
			ErrField:         "resourceClaims.resourceClaimTemplateName=my-template",
		},
		{
			Name:             "resourceClaims.resourceClaimTemplateName restricted by any value",
			Override:         corev1.PodSpec{ResourceClaims: []corev1.PodResourceClaim{{Name: "gpu", ResourceClaimTemplateName: ptr.To("my-template")}}},
			RestrictedFields: []string{"resourceClaims.resourceClaimTemplateName"},
			IsErrorExpected:  true,
			ErrField:         "resourceClaims.resourceClaimTemplateName",
		},
		{
			Name:             "resourceClaims.resourceClaimTemplateName allowed when restricted value does not match",
			Override:         corev1.PodSpec{ResourceClaims: []corev1.PodResourceClaim{{Name: "gpu", ResourceClaimTemplateName: ptr.To("my-template")}}},
			RestrictedFields: []string{"resourceClaims.resourceClaimTemplateName=other-template"},
			IsErrorExpected:  false,
		},
		// ----------- Resources -----------
		{
			Name:             "resources nil not restricted",
			Override:         corev1.PodSpec{Resources: nil},
			RestrictedFields: []string{"resources"},
			IsErrorExpected:  false,
		},
		{
			Name:             "resources restricted by any value",
			Override:         corev1.PodSpec{Resources: &corev1.ResourceRequirements{}},
			RestrictedFields: []string{"resources"},
			IsErrorExpected:  true,
			ErrField:         "resources",
		},
		// ----------- Resources.Limits -----------
		{
			Name:             "resources.limits nil not restricted",
			Override:         corev1.PodSpec{Resources: &corev1.ResourceRequirements{Limits: nil}},
			RestrictedFields: []string{"resources.limits"},
			IsErrorExpected:  false,
		},
		{
			Name:             "resources.limits restricted by any value",
			Override:         corev1.PodSpec{Resources: &corev1.ResourceRequirements{Limits: corev1.ResourceList{}}},
			RestrictedFields: []string{"resources.limits"},
			IsErrorExpected:  true,
			ErrField:         "resources.limits",
		},
		// ----------- Resources.Requests -----------
		{
			Name:             "resources.requests nil not restricted",
			Override:         corev1.PodSpec{Resources: &corev1.ResourceRequirements{Requests: nil}},
			RestrictedFields: []string{"resources.requests"},
			IsErrorExpected:  false,
		},
		{
			Name:             "resources.requests restricted by any value",
			Override:         corev1.PodSpec{Resources: &corev1.ResourceRequirements{Requests: corev1.ResourceList{}}},
			RestrictedFields: []string{"resources.requests"},
			IsErrorExpected:  true,
			ErrField:         "resources.requests",
		},
		// ----------- Resources.Claims -----------
		{
			Name:             "resources.claims empty not restricted",
			Override:         corev1.PodSpec{Resources: &corev1.ResourceRequirements{Claims: nil}},
			RestrictedFields: []string{"resources.claims"},
			IsErrorExpected:  false,
		},
		{
			Name:             "resources.claims restricted by any value",
			Override:         corev1.PodSpec{Resources: &corev1.ResourceRequirements{Claims: []corev1.ResourceClaim{{Name: "gpu"}}}},
			RestrictedFields: []string{"resources.claims"},
			IsErrorExpected:  true,
			ErrField:         "resources.claims",
		},
		// ----------- Resources.Limits.CPU -----------
		{
			Name:             "resources.limits.cpu restricted by any value",
			Override:         corev1.PodSpec{Resources: &corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}}},
			RestrictedFields: []string{"resources.limits.cpu"},
			IsErrorExpected:  true,
			ErrField:         "resources.limits.cpu",
		},
		// ----------- Resources.Limits.Memory -----------
		{
			Name:             "resources.limits.memory restricted by any value",
			Override:         corev1.PodSpec{Resources: &corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")}}},
			RestrictedFields: []string{"resources.limits.memory"},
			IsErrorExpected:  true,
			ErrField:         "resources.limits.memory",
		},
		// ----------- Resources.Requests.CPU -----------
		{
			Name:             "resources.requests.cpu restricted by any value",
			Override:         corev1.PodSpec{Resources: &corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m")}}},
			RestrictedFields: []string{"resources.requests.cpu"},
			IsErrorExpected:  true,
			ErrField:         "resources.requests.cpu",
		},
		// ----------- Resources.Requests.Memory -----------
		{
			Name:             "resources.requests.memory restricted by any value",
			Override:         corev1.PodSpec{Resources: &corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("128Mi")}}},
			RestrictedFields: []string{"resources.requests.memory"},
			IsErrorExpected:  true,
			ErrField:         "resources.requests.memory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			err := restrictPodOverride(&tt.Override, tt.RestrictedFields)

			if tt.IsErrorExpected {
				assert.Error(t, err)
				assert.Equal(t, getPodRestrictionErr(tt.ErrField).Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestApplyPodOverridesStripsUnknownFields(t *testing.T) {
	overrideJSON := `{"spec":{"schedulerName":"custom","futureSecurityField":"malicious-value","unknownNested":{"key":"val"}}}`

	workspace := &common.DevWorkspaceWithConfig{}
	workspace.DevWorkspace = &dw.DevWorkspace{}
	workspace.Spec.Template = dw.DevWorkspaceTemplateSpec{
		DevWorkspaceTemplateSpecContent: dw.DevWorkspaceTemplateSpecContent{
			Attributes: attributes.Attributes{
				constants.PodOverridesAttribute: apiext.JSON{Raw: []byte(overrideJSON)},
			},
			Components: []dw.Component{{
				Name: "test-component",
				ComponentUnion: dw.ComponentUnion{
					Container: &dw.ContainerComponent{
						Container: dw.Container{Image: "test-image"},
					},
				},
			}},
		},
	}

	deployment := &appsv1.Deployment{}
	deployment.Spec.Template.Spec.Containers = []corev1.Container{{
		Name:  "test-component",
		Image: "test-image",
	}}

	patched, err := ApplyPodOverrides(workspace, deployment)
	assert.NoError(t, err)
	assert.Equal(t, "custom", patched.Spec.Template.Spec.SchedulerName)

	patchedBytes, err := json.Marshal(patched.Spec.Template.Spec)
	assert.NoError(t, err)
	assert.NotContains(t, string(patchedBytes), "futureSecurityField")
	assert.NotContains(t, string(patchedBytes), "unknownNested")
}

func preemptionPolicyPtr(p corev1.PreemptionPolicy) *corev1.PreemptionPolicy {
	return &p
}

func supplementalGroupsPolicyPtr(p corev1.SupplementalGroupsPolicy) *corev1.SupplementalGroupsPolicy {
	return &p
}

func fsGroupChangePolicyPtr(p corev1.PodFSGroupChangePolicy) *corev1.PodFSGroupChangePolicy {
	return &p
}

func seLinuxChangePolicyPtr(p corev1.PodSELinuxChangePolicy) *corev1.PodSELinuxChangePolicy {
	return &p
}
