module github.com/devfile/devworkspace-operator

go 1.16

require (
	github.com/devfile/api/v2 v2.0.0-20220414122024-32cae1f8e42c
	github.com/go-git/go-git/v5 v5.2.0
	github.com/go-logr/logr v0.4.0
	github.com/google/go-cmp v0.5.5
	github.com/google/gofuzz v1.2.0
	github.com/google/uuid v1.1.2
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.14.0
	github.com/openshift/api v0.0.0-20200205133042-34f0ec8dab87
	github.com/prometheus/client_golang v1.11.0
	github.com/redhat-cop/operator-utils v1.1.4
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	golang.org/x/net v0.0.0-20210428140749-89ef3d95e781
	k8s.io/api v0.21.3
	k8s.io/apiextensions-apiserver v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	sigs.k8s.io/controller-runtime v0.9.5
	sigs.k8s.io/yaml v1.2.0
)
