# Cert Manager DeSEC Webhook

<p>
  <img src="https://raw.githubusercontent.com/cert-manager/cert-manager/d53c0b9270f8cd90d908460d69502694e1838f5f/logo/logo-small.png" height="256" width="256" alt="cert-manager project logo" />
</p>

# Independently maintained ACME webhook for desec.io DNS API

This solver can be used with [desec.io](https://desec.io) DNS API. The documentation of the API can be found [here](https://desec.readthedocs.io/en/latest/)

## Requirements
- [go](https://golang.org) => 1.26.0
- [helm](https://helm.sh/) >= v3.0.0
- [kuberentes](https://kubernetes.io/) => 1.25.0
- [cert-manager](https://cert-manager.io/) => 1.19.0

## Installation

### Using helm from local checkout
```bash
helm install \
  -n cert-manager \
  desec-webhook \
  charts/cert-manager-desec-webhook
```

### Using public helm chart
```bash
helm install \
  -n cert-manager \
  --version <release without leading "v"> \
  desec-webhook \
  oci://ghcr.io/pr0ton11/charts/cert-manager-desec-webhook
```

## Uninstallation

## Creating an issuer

Create a secret containing the credentials
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: desec-io-token
  namespace: cert-manager
type: Opaque
data:
  token: your-key-base64-encoded
```

We can also then provide a standardised 'testing framework', or set of
conformance tests, which allow us to validate that a DNS provider works as
expected.
Create a 'ClusterIssuer' or 'Issuer' resource as the following:

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
spec:
  acme:
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    email: mail@example.com

    privateKeySecretRef:
      name: letsencrypt-staging

    solvers:
      - dns01:
          webhook:
            config:
              apiKeySecretRef:
                key: token
                name: desec-io-token
            groupName: acme.pr0ton11.github.com
            solverName: desec
```

## Create a manual certificate

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: example-cert
  namespace: cert-manager
spec:
  commonName: example.com
  dnsNames:
    - example.com
  issuerRef:
    name: letsencrypt-staging
    kind: ClusterIssuer
  secretName: example-cert
```

## Using cert-manager with traefik ingress
```yaml

apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: bitwarden
  namespace: utils
  labels:
    app: bitwarden
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-staging
    kubernetes.io/ingress.class: traefik
    traefik.ingress.kubernetes.io/rewrite-target: /$1
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    traefik.ingress.kubernetes.io/router.tls: 'true'
spec:
  tls:
    - hosts:
        - bitwarden.acme.example.com
      secretName: bitwarden-crt
  rules:
    - host: bitwarden.acme.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: bitwarden
                port:
                  number: 80

```

### Creating your own repository

### Running the test suite

All DNS providers **must** run the DNS01 provider conformance testing suite,
else they will have undetermined behaviour when used with cert-manager.

Provide a secret.yaml in testdata/desec

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: desec-token
data:
  token: your-key-base64-encoded
type: Opaque
```

Define a **TEST_ZONE_NAME** matching to your authenticaton creditials.

```bash
$ TEST_ZONE_NAME=example.com. make test
```
