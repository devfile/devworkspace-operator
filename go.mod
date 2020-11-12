module github.com/devfile/devworkspace-operator

go 1.13

require (
	github.com/apex/log v1.9.0
	github.com/devfile/api v0.0.0-20200826083800-9e2280a95680
	github.com/eclipse/che-go-jsonrpc v0.0.0-20200317130110-931966b891fe // indirect
	github.com/eclipse/che-plugin-broker v3.4.0+incompatible
	github.com/go-logr/logr v0.1.0
	github.com/google/go-cmp v0.5.0
	github.com/google/uuid v1.1.1
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/openshift/api v0.0.0-20200205133042-34f0ec8dab87
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.16.0 // indirect
	golang.org/x/lint v0.0.0-20191125180803-fdd1cda4f05f // indirect
	golang.org/x/net v0.0.0-20200625001655-4c5254603344 // indirect
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae // indirect
	golang.org/x/tools v0.0.0-20200403190813-44a64ad78b9b // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	k8s.io/api v0.18.8
	k8s.io/apiextensions-apiserver v0.18.8 // indirect
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/yaml v1.2.0
)

// devfile/api requires v12.0.0+incompatible but this causes issues with go commands
replace k8s.io/client-go => k8s.io/client-go v0.18.6

replace github.com/devfile/api => github.com/amisevsk/devworkspace-api v0.0.0-20201020205654-257362dba943
