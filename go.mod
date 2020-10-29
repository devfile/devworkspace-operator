module github.com/devfile/devworkspace-operator

go 1.13

require (
	github.com/apex/log v1.9.0
	github.com/devfile/api v0.0.0-20200826083800-9e2280a95680
	github.com/eclipse/che-go-jsonrpc v0.0.0-20200317130110-931966b891fe // indirect
	github.com/eclipse/che-plugin-broker v3.4.0+incompatible
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-logr/logr v0.1.0
	github.com/google/go-cmp v0.4.0
	github.com/google/uuid v1.1.1
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/openshift/api v0.0.0-20200205133042-34f0ec8dab87
	github.com/operator-framework/operator-sdk v0.17.0
	github.com/stretchr/testify v1.6.1
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.2
)

// devfile/api requires v12.0.0+incompatible but this causes issues with go commands
replace k8s.io/client-go => k8s.io/client-go v0.18.6

replace github.com/devfile/api => github.com/amisevsk/devworkspace-api v0.0.0-20201020205654-257362dba943
