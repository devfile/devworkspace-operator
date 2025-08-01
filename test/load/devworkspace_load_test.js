//
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
//

import http from 'k6/http';
import {check, sleep} from 'k6';
import {Trend, Counter} from 'k6/metrics';
import {htmlReport} from "https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js";
import {textSummary} from "https://jslib.k6.io/k6-summary/0.0.1/index.js";

const inCluster = __ENV.IN_CLUSTER === 'true';
const apiServer = inCluster ? `https://kubernetes.default.svc` : __ENV.KUBE_API;
const token = inCluster ? open('/var/run/secrets/kubernetes.io/serviceaccount/token') : __ENV.KUBE_TOKEN;
const useSeparateNamespaces = __ENV.SEPARATE_NAMESPACES === "true";
const operatorNamespace = __ENV.DWO_NAMESPACE || 'openshift-operators';
const externalDevWorkspaceLink = __ENV.DEVWORKSPACE_LINK || '';
const shouldCreateAutomountResources = (__ENV.CREATE_AUTOMOUNT_RESOURCES || 'false') === 'true';
const maxVUs = Number(__ENV.MAX_VUS || 50);
const devWorkspaceReadyTimeout = Number(__ENV.DEV_WORKSPACE_READY_TIMEOUT_IN_SECONDS || 600);
const autoMountConfigMapName = 'dwo-load-test-automount-configmap';
const autoMountSecretName = 'dwo-load-test-automount-secret';
const labelType = "test-type";
const labelKey = "load-test";
const loadTestDurationInMinutes = __ENV.TEST_DURATION_IN_MINUTES || "25";
const loadTestNamespace = __ENV.LOAD_TEST_NAMESPACE || "loadtest-devworkspaces";

const headers = {
  Authorization: `Bearer ${token}`, 'Content-Type': 'application/json',
};

export const options = {
  scenarios: {
    create_and_delete_devworkspaces: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: generateLoadTestStages(maxVUs),
      gracefulRampDown: '1m',
    },
    final_cleanup: {
      executor: 'per-vu-iterations',
      vus: 1,
      iterations: 1,
      startTime: `${loadTestDurationInMinutes}m`,
      exec: 'final_cleanup',
    },
  }, thresholds: {
    'checks': ['rate>0.95'],
    'devworkspace_create_duration': ['p(95)<15000'],
    'devworkspace_delete_duration': ['p(95)<10000'],
    'devworkspace_ready_duration': ['p(95)<60000'],
    'devworkspace_ready_failed': ['count<5'],
    'operator_cpu_violations': ['count==0'],
    'operator_mem_violations': ['count==0'],
  }, insecureSkipTLSVerify: true,  // trust self-signed certs like in CRC
};

const devworkspaceCreateDuration = new Trend('devworkspace_create_duration');
const devworkspaceReady = new Counter('devworkspace_ready');
const devworkspaceDeleteDuration = new Trend('devworkspace_delete_duration');
const devworkspaceReadyDuration = new Trend('devworkspace_ready_duration');
const devworkspaceReadyFailed = new Counter('devworkspace_ready_failed');
const operatorCpu = new Trend('average_operator_cpu'); // in milli cores
const operatorMemory = new Trend('average_operator_memory'); // in Mi
const devworkspacesCreated = new Counter('devworkspace_create_count');
const operatorCpuViolations = new Counter('operator_cpu_violations');
const operatorMemViolations = new Counter('operator_mem_violations');

const maxCpuMillicores = 250;
const maxMemoryBytes = 200 * 1024 * 1024;

export function setup() {
  if (shouldCreateAutomountResources) {
    createNewAutomountConfigMap();
    createNewAutomountSecret();
  }
}

export default function () {
  const vuId = __VU;
  const iteration = __ITER;
  const crName = `dw-test-${vuId}-${iteration}`;
  const namespace = useSeparateNamespaces
      ? `load-test-ns-${__VU}-${__ITER}`
      : loadTestNamespace;

  if (!apiServer) {
    throw new Error('KUBE_API env var is required');
  }
  try {
    if (useSeparateNamespaces) {
      createNewNamespace(namespace);
    }
    const devWorkspaceCreated = createNewDevWorkspace(namespace, vuId, iteration);
    if (devWorkspaceCreated) {
      waitUntilDevWorkspaceIsReady(vuId, crName, namespace);
      deleteDevWorkspace(crName, namespace);
    }
  } catch (error) {
    console.error(`Load test for ${vuId}-${iteration} failed:`, error.message);
  }
}

export function final_cleanup() {
  if (useSeparateNamespaces) {
    deleteAllSeparateNamespaces();
  } else {
    deleteAllDevWorkspacesInCurrentNamespace();
  }

  if (shouldCreateAutomountResources) {
    deleteConfigMap();
    deleteSecret();
  }
}

export function handleSummary(data) {
  const allowed = ['devworkspace_create_count', 'devworkspace_create_duration', 'devworkspace_delete_duration', 'devworkspace_ready_duration', 'devworkspace_ready', 'devworkspace_ready_failed', 'operator_cpu_violations', 'operator_mem_violations', 'average_operator_cpu', 'average_operator_memory'];

  const filteredData = JSON.parse(JSON.stringify(data));
  for (const key of Object.keys(filteredData.metrics)) {
    if (!allowed.includes(key)) {
      delete filteredData.metrics[key];
    }
  }

  let loadTestSummaryReport = {
    stdout: textSummary(filteredData, {indent: ' ', enableColors: true})
  }
  // Only generate HTML report when running outside the cluster
  if (!inCluster) {
    loadTestSummaryReport["devworkspace-load-test-report.html"] = htmlReport(data, {
      title: "DevWorkspace Operator Load Test Report (HTTP)",
    });
  }
  return loadTestSummaryReport;
}

function createNewDevWorkspace(namespace, vuId, iteration) {
  const baseUrl = `${apiServer}/apis/workspace.devfile.io/v1alpha2/namespaces/${namespace}/devworkspaces`;

  const manifest = generateDevWorkspaceToCreate(vuId, iteration, namespace);

  const payload = JSON.stringify(manifest);

  const createStart = Date.now();
  const createRes = http.post(baseUrl, payload, {headers});
  check(createRes, {
    'DevWorkspace created': (r) => r.status === 201 || r.status === 409,
  });

  if (createRes.status !== 201 && createRes.status !== 409) {
    console.error(`[VU ${vuId}] Failed to create DevWorkspace: ${createRes.status}, ${createRes.body}`);
    return false;
  }
  devworkspaceCreateDuration.add(Date.now() - createStart);
  devworkspacesCreated.add(1);
  return true;
}

function waitUntilDevWorkspaceIsReady(vuId, crName, namespace) {
  const dwUrl = `${apiServer}/apis/workspace.devfile.io/v1alpha2/namespaces/${namespace}/devworkspaces/${crName}`;
  const readyStart = Date.now();
  let isReady = false;
  let attempts = 0;
  const pollWaitInterval = 5;
  const maxAttempts = devWorkspaceReadyTimeout / pollWaitInterval;
  let res = {};

  while (!isReady && attempts < maxAttempts) {
    res = http.get(`${dwUrl}`, {headers});

    if (res.status === 200) {
      try {
        const body = JSON.parse(res.body);
        const phase = body?.status?.phase;
        if (phase === 'Ready' || phase === 'Running') {
          isReady = true;
          break;
        } else if (phase === 'Failing' || phase === 'Failed' || phase === 'Error') {
          isReady = false;
          break;
        }
      } catch (e) {
        console.error(`GET [VU ${vuId}] Failed to parse DevWorkspace from API: ${res.body} : ${e.message}`);
      }
    }

    checkDevWorkspaceOperatorMetrics();
    sleep(pollWaitInterval);
    attempts++;
  }

  if (res.status === 200) {
    if (isReady) {
      devworkspaceReady.add(1);
      devworkspaceReadyDuration.add(Date.now() - readyStart);
    } else {
      devworkspaceReadyFailed.add(1);
    }
  }
}

function deleteDevWorkspace(crName, namespace) {
  const dwUrl = `${apiServer}/apis/workspace.devfile.io/v1alpha2/namespaces/${namespace}/devworkspaces/${crName}`;
  const deleteStart = Date.now();
  const delRes = http.del(`${dwUrl}`, null, {headers});
  devworkspaceDeleteDuration.add(Date.now() - deleteStart);

  check(delRes, {
    'DevWorkspace deleted or not found': (r) => r.status === 200 || r.status === 404,
  });
}

function checkDevWorkspaceOperatorMetrics() {
  const metricsUrl = `${apiServer}/apis/metrics.k8s.io/v1beta1/namespaces/${operatorNamespace}/pods`;
  const res = http.get(metricsUrl, {headers});


  check(res, {
    'Fetched pod metrics successfully': (r) => r.status === 200,
  });

  if (res.status !== 200) {
    console.warn(`[DWO METRICS] Unable to fetch DevWorkspace Operator metrics from Kubernetes, got ${res.status}`);
    return;
  }

  const data = JSON.parse(res.body);
  const operatorPods = data.items.filter(p => p.metadata.name.includes("devworkspace-controller"));

  for (const pod of operatorPods) {
    const container = pod.containers[0]; // assuming single container
    const name = pod.metadata.name;

    const cpu = parseCpuToMillicores(container.usage.cpu);
    const memory = parseMemoryToBytes(container.usage.memory);

    operatorCpu.add(cpu);
    operatorMemory.add(memory / 1024 / 1024);

    const cpuOk = cpu <= maxCpuMillicores;
    const memOk = memory <= maxMemoryBytes;

    if (!cpuOk) {
      operatorCpuViolations.add(1);
    }
    if (!memOk) {
      operatorMemViolations.add(1);
    }

    check(null, {
      [`[${name}] CPU < ${maxCpuMillicores}m`]: () => cpuOk,
      [`[${name}] Memory < ${Math.round(maxMemoryBytes / 1024 / 1024)}Mi`]: () => memOk,
    });
  }
}

function createNewNamespace(namespaceName) {
  const url = `${apiServer}/api/v1/namespaces`;

  const namespaceObj = {
    apiVersion: 'v1',
    kind: 'Namespace',
    metadata: {
      name: namespaceName,
      labels: {
        [labelKey]: labelType
      }
    }
  }
  const res = http.post(url, JSON.stringify(namespaceObj), {headers});

  if (res.status !== 201 && res.status !== 409) {
    throw new Error(`Failed to create Namespace: ${res.status} - ${namespaceName}`);
  }
}

function createNewAutomountConfigMap() {
  const url = `${apiServer}/api/v1/namespaces/${loadTestNamespace}/configmaps`;

  const configMapManifest = {
    apiVersion: 'v1', kind: 'ConfigMap', metadata: {
      name: autoMountConfigMapName, namespace: loadTestNamespace, labels: {
        'controller.devfile.io/mount-to-devworkspace': 'true', 'controller.devfile.io/watch-configmap': 'true',
      }, annotations: {
        'controller.devfile.io/mount-path': '/etc/config/dwo-load-test-configmap',
        'controller.devfile.io/mount-access-mode': '0644',
        'controller.devfile.io/mount-as': 'file',
      },
    }, data: {
      'test.key': 'test-value',
    },
  };

  const res = http.post(url, JSON.stringify(configMapManifest), {headers});

  if (res.status !== 201 && res.status !== 409) {
    throw new Error(`Failed to create automount ConfigMap: ${res.status} - ${res.body}`);
  }
  console.log("Created automount configMap : " + autoMountConfigMapName);
}

function createNewAutomountSecret() {
  const manifest = {
    apiVersion: 'v1', kind: 'Secret', metadata: {
      name: autoMountSecretName, namespace: loadTestNamespace, labels: {
        'controller.devfile.io/mount-to-devworkspace': 'true', 'controller.devfile.io/watch-secret': 'true',
      }, annotations: {
        'controller.devfile.io/mount-path': `/etc/secret/dwo-load-test-secret`,
        'controller.devfile.io/mount-as': 'file',
      },
    }, type: 'Opaque', data: {
      'secret.key': __ENV.SECRET_VALUE_BASE64 || 'dGVzdA==', // base64-encoded 'test'
    },
  };

  const res = http.post(`${apiServer}/api/v1/namespaces/${loadTestNamespace}/secrets`, JSON.stringify(manifest), {headers});
  if (res.status !== 201 && res.status !== 409) {
    throw new Error(`Failed to create automount Secret: ${res.status} - ${res.body}`);
  }
}

function deleteConfigMap() {
  const url = `${apiServer}/api/v1/namespaces/${loadTestNamespace}/configmaps/${autoMountConfigMapName}`;
  const res = http.del(url, null, { headers });
  if (res.status !== 200 && res.status !== 404) {
    console.warn(`[CLEANUP] Failed to delete ConfigMap ${autoMountConfigMapName}: ${res.status}`);
  }
}

function deleteSecret() {
  const url = `${apiServer}/api/v1/namespaces/${loadTestNamespace}/secrets/${autoMountSecretName}`;
  const res = http.del(url, null, {headers});
  if (res.status !== 200 && res.status !== 404) {
    console.warn(`[CLEANUP] Failed to delete Secret ${autoMountSecretName}: ${res.status}`);
  }
}

function deleteNamespace(name) {
  const delRes = http.del(`${apiServer}/api/v1/namespaces/${name}`, null, {headers});
  if (delRes.status !== 200 && delRes.status !== 404) {
    console.warn(`[CLEANUP] Failed to delete Namespace ${name}: ${delRes.status}`);
  }
}

function deleteAllDevWorkspacesInCurrentNamespace() {
  const deleteByLabelSelectorUrl = `${apiServer}/apis/workspace.devfile.io/v1alpha2/namespaces/${loadTestNamespace}/devworkspaces?labelSelector=${labelKey}%3D${labelType}`;
  console.log(`[CLEANUP] Deleting all DevWorkspaces in ${loadTestNamespace} containing label ${labelKey}=${labelType}`);

  const res = http.del(deleteByLabelSelectorUrl, null, {headers});
  if (res.status !== 200) {
    console.error(`[CLEANUP] Failed to delete DevWorkspaces: ${res.status}`);
  }
}

function deleteAllSeparateNamespaces() {
  const getNamespacesByLabel = `${apiServer}/api/v1/namespaces?labelSelector=${labelKey}%3D${labelType}`;
  console.log(`[CLEANUP] Deleting all Namespaces containing label ${labelKey}=${labelType}`);

  const res = http.get(getNamespacesByLabel, {headers});
  if (res.status !== 200) {
    console.error(`[CLEANUP] Failed to list DevWorkspaces: ${res.status}`);
    return;
  }

  const body = JSON.parse(res.body);
  if (!body.items || !Array.isArray(body.items)) return;

  for (const item of body.items) {
    deleteNamespace(item.metadata.name);
  }
}

function createOpinionatedDevWorkspace() {
  return {
    apiVersion: "workspace.devfile.io/v1alpha2", kind: "DevWorkspace", metadata: {
      name: "minimal-dw",
      namespace: loadTestNamespace,
      labels: {
        [labelKey]: labelType
      }
    }, spec: {
      started: true, template: {
        attributes: {
          "controller.devfile.io/storage-type": "ephemeral",
        }, components: [{
          name: "dev", container: {
            image: "registry.access.redhat.com/ubi9/ubi-micro:9.6-1752751762",
            command: ["sleep", "3600"],
            imagePullPolicy: "IfNotPresent",
            memoryLimit: "64Mi",
            memoryRequest: "32Mi",
            cpuLimit: "200m",
            cpuRequest: "100m"
          },
        },],
      },
    },
  };
}

function parseJSONResponseToDevWorkspace(response) {
  let devWorkspace;
  try {
    devWorkspace = response.json();
  } catch (e) {
    throw new Error(`[DW CREATE] Failed to parse JSON : ${response.body}: ${e.message}`);
  }
  return devWorkspace;
}

function downloadAndParseExternalWorkspace(externalDevWorkspaceLink) {
  let manifest;
  if (externalDevWorkspaceLink) {
    const res = http.get(externalDevWorkspaceLink);

    if (res.status !== 200) {
      throw new Error(`[DW CREATE] Failed to fetch JSON content from ${externalDevWorkspaceLink}, got ${res.status}`);
    }
    manifest = parseJSONResponseToDevWorkspace(res);
  }

  return manifest;
}

function generateDevWorkspaceToCreate(vuId, iteration, namespace) {
  const name = `dw-test-${vuId}-${iteration}`;
  let devWorkspace = {};
  if (externalDevWorkspaceLink.length > 0) {
    devWorkspace = downloadAndParseExternalWorkspace(externalDevWorkspaceLink);
  } else {
    devWorkspace = createOpinionatedDevWorkspace();
  }
  devWorkspace.metadata.name = name;
  devWorkspace.metadata.namespace = namespace;
  devWorkspace.metadata.labels = {
    [labelKey]: labelType
  }
  return devWorkspace;
}

function generateLoadTestStages(max) {
  const stageDefinitions = [
    { percent: 0.25, target: Math.floor(max * 0.25) },
    { percent: 0.25, target: Math.floor(max * 0.5) },
    { percent: 0.2, target: Math.floor(max * 0.75) },
    { percent: 0.15, target: max },
    { percent: 0.10, target: Math.floor(max * 0.5) },
    { percent: 0.05, target: 0 },
  ];

  return stageDefinitions.map(({ percent, target }) => ({
    duration: `${Math.round(loadTestDurationInMinutes * percent)}m`,
    target,
  }));
}

function parseMemoryToBytes(memStr) {
  if (memStr.endsWith("Ki")) return parseInt(memStr) * 1024;
  if (memStr.endsWith("Mi")) return parseInt(memStr) * 1024 * 1024;
  if (memStr.endsWith("Gi")) return parseInt(memStr) * 1024 * 1024 * 1024;
  if (memStr.endsWith("n")) return parseInt(memStr) / 1e9;
  if (memStr.endsWith("u")) return parseInt(memStr) / 1e6;
  if (memStr.endsWith("m")) return parseInt(memStr) / 1e3;
  return parseInt(memStr); // bytes
}

function parseCpuToMillicores(cpuStr) {
  if (cpuStr.endsWith("n")) return Math.round(parseInt(cpuStr) / 1e6);
  if (cpuStr.endsWith("u")) return Math.round(parseInt(cpuStr) / 1e3);
  if (cpuStr.endsWith("m")) return parseInt(cpuStr);
  return Math.round(parseFloat(cpuStr) * 1000);
}