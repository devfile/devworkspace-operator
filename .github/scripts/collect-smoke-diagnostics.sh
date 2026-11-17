#!/bin/bash
#
# Copyright (c) 2019-2026 Red Hat, Inc.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# Dump the objects that explain a failed load-test smoke run. Invoked with
# `if: failure()`, so a missing cluster must not fail the step.
set +e

namespace="${DWO_NAMESPACE:-devworkspace-controller}"

echo "===== pods ====="
kubectl get pods -A -o wide

echo "===== controller logs ====="
kubectl logs -n "${namespace}" -l app.kubernetes.io/name=devworkspace-controller --tail=200

echo "===== webhook logs ====="
kubectl logs -n "${namespace}" -l app.kubernetes.io/name=devworkspace-webhook-server --tail=200

echo "===== devworkspaces ====="
kubectl get devworkspaces -A -o yaml

echo "===== devworkspaceoperatorconfigs ====="
kubectl get devworkspaceoperatorconfigs -A -o yaml

echo "===== backup jobs ====="
kubectl get jobs -A -l controller.devfile.io/backup-job=true -o yaml

echo "===== warning events ====="
kubectl get events -A --field-selector type=Warning

exit 0
