module github.com/devfile/devworkspace-operator

go 1.15

require (
	github.com/devfile/api/v2 v2.0.0-20210713124824-03e023e7078b
	github.com/go-git/go-git/v5 v5.2.0
	github.com/go-logr/logr v0.3.0
	github.com/go-logr/zapr v0.3.0 // indirect
	github.com/google/go-cmp v0.5.2
	github.com/google/uuid v1.1.2
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/openshift/api v0.0.0-20200205133042-34f0ec8dab87
	github.com/prometheus/client_golang v1.7.1
	github.com/redhat-cop/operator-utils v0.1.0
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.16.0 // indirect
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0
	gopkg.in/yaml.v3 v3.0.0-20200605160147-a5ece683394c // indirect
	k8s.io/api v0.20.6
	k8s.io/apiextensions-apiserver v0.20.6 // indirect
	k8s.io/apimachinery v0.20.6
	k8s.io/client-go v0.20.6
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/yaml v1.2.0
)
