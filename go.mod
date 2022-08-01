module github.com/pluralsh/plural-certmanager-webhook

go 1.13

require (
	github.com/cert-manager/webhook-example v0.0.0-20210224141901-9440683e53e1
	github.com/jetstack/cert-manager v1.2.0
	github.com/miekg/dns v1.1.31
	github.com/pluralsh/gqlclient v1.0.9
	github.com/stretchr/testify v1.7.2
	k8s.io/apiextensions-apiserver v0.19.0
	k8s.io/client-go v0.19.0
)

replace github.com/Yamashou/gqlgenc => github.com/pluralsh/gqlgenc v0.0.9
