module github.com/che-incubator/che-workspace-operator

go 1.12

// Che Plugin Broker branch devworkspace-controller
require github.com/eclipse/che-plugin-broker v3.1.1-0.20200207223144-b20597f15e4c+incompatible

// use-devfile-2.0-in-workspace-controller
require github.com/devfile/kubernetes-api v0.0.0-20200608141605-bccde9afdf59

// Operator Framework 0.17.x
require (
	github.com/eclipse/che-go-jsonrpc v0.0.0-20181205102516-87cdb8da2597 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/google/go-cmp v0.4.0
	github.com/google/uuid v1.1.1
	github.com/openshift/api v3.9.1-0.20190424152011-77b8897ec79a+incompatible
	gopkg.in/yaml.v2 v2.2.8
)

require (
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/operator-framework/operator-sdk v0.17.0
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.5.2
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.17.4 // Required by prometheus-operator
)
