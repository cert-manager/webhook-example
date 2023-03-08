package server

import (
	"github.com/pluralsh/acme/pkg/apis/v1alpha1/acme"
	pkgruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func init() {
	utilruntime.Must(acme.AddToScheme(scheme))
	utilruntime.Must(scheme.SetVersionPriority(acme.SchemeGroupVersion))
}

var scheme = pkgruntime.NewScheme()
