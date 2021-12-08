# Uninstalling the DevWorkspace Operator
The DevWorkspace Operator makes use of finalizers on DevWorkspace resources and webhooks to restrict access to development resources. As these are not automatically removed when the Operator is uninstalled, they must manually be removed to fully clean the operator from the cluster.

**Note:** The DevWorkspace Operator utilizes validating webhooks to restict `pods/exec` access to workspace resources. As Kubernetes provides no way to restrict which pods these webhooks apply to, improper removal of the webhook server could result in exec access to all pods on the cluster being blocked.

1. Ensure that all DevWorkspace Custom Resources are removed along with their related k8s objects, like deployments.

	```
	kubectl delete devworkspaces.workspace.devfile.io --all-namespaces --all --wait
	kubectl delete devworkspaceroutings.controller.devfile.io --all-namespaces --all --wait
	```
	Note: This step must be done first, as otherwise the resources above may have finalizers that block automatic cleanup.

2. Uninstall the Operator

3. Remove the custom resource definitions installed by the operator

	```
	kubectl delete customresourcedefinitions.apiextensions.k8s.io devworkspaceroutings.controller.devfile.io
	kubectl delete customresourcedefinitions.apiextensions.k8s.io devworkspaces.workspace.devfile.io
	kubectl delete customresourcedefinitions.apiextensions.k8s.io devworkspacetemplates.workspace.devfile.io
	```

4. Remove DevWorkspace Webhook Server deployment and mutating/validating webhook configurations

	```
	kubectl delete deployment/devworkspace-webhook-server -n openshift-operators
  kubectl delete mutatingwebhookconfigurations controller.devfile.io
	kubectl delete validatingwebhookconfigurations controller.devfile.io
	```

5. Remove lingering service, secrets, and configmaps

	```
	kubectl delete all --selector app.kubernetes.io/part-of=devworkspace-operator
	kubectl delete serviceaccounts devworkspace-webhook-server -n openshift-operators
	kubectl delete configmap devworkspace-controller -n openshift-operators
	kubectl delete clusterrole devworkspace-webhook-server
	kubectl delete clusterrolebinding devworkspace-webhook-server
	```
