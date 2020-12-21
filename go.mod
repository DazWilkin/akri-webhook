module github.com/deislabs/akri/webhook

go 1.15

require (
	k8s.io/api v0.20.1
	k8s.io/apiextensions-apiserver v0.20.1
	k8s.io/apimachinery v0.20.1
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog v1.0.0
)

replace k8s.io/client-go => k8s.io/client-go v0.20.1
