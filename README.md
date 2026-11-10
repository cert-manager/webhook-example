# cert-manager-webhook-123domain

cert-manager DNS01 webhook for managing 123domain.eu DNS TXT records through
the [123domain.eu API](https://api.123domain.eu/spec/swagger.json).

> **Disclaimer (2026-11-10):** This is an unofficial, community-maintained
> project and is not affiliated with or endorsed by 123domain.eu. It is
> provided as-is; the maintainers are not responsible for any damages, losses,
> or issues arising from its use.

## 123domain.eu API integration

The webhook solver name is `123domain`. Presenting a challenge creates the
matching TXT record if it does not already exist. Cleanup removes only the TXT
record matching that challenge's name and value.

Create one Kubernetes Secret named `ns123domain-api-secret` in the namespace
containing the Issuer (or cert-manager's cluster resource namespace for a
ClusterIssuer). Store both credentials as `api-key` and `api-secret` entries:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ns123domain-api-secret
type: Opaque
stringData:
  api-key: "<your-123domain-api-key>"
  api-secret: "<your-123domain-api-secret>"
```

Reference the Secret from the webhook solver configuration. The config
property is named `apiSecretKeyRef`; the Secret data keys are fixed as
`api-key` and `api-secret`:

```yaml
config:
  apiSecretKeyRef:
    name: ns123domain-api-secret
```

The chart grants the webhook service account `get` access to the named Secret
in the chart release namespace by default. Override
`credentialsSecret.namespace` when the Secret is in a different namespace. For
a ClusterIssuer, this is typically cert-manager's configured cluster resource
namespace. Configure the chart's `groupName` to a unique DNS name and use the
same value in the Issuer configuration.

The chart defaults to the `ghcr.io/ru13en/cert-manager-webhook-123domain`
image repository. Install it after publishing the image, overriding the tag
when needed:

```sh
helm upgrade --install cert-manager-webhook-123domain \
  ./deploy/cert-manager-webhook-123domain \
  --namespace cert-manager \
  --set groupName=acme.github.ru13en.123domain.eu \
  --set image.tag=v0.1.0 \
  --set credentialsSecret.namespace=issuer-namespace
```

The chart runs the webhook as a non-root user and supports configurable
probes, scheduling, extra volumes, a metrics endpoint, an optional
Prometheus Operator `ServiceMonitor`, and an optional PodDisruptionBudget.
See [`values.yaml`](./deploy/cert-manager-webhook-123domain/values.yaml) for
the available settings.

Example `Issuer` configuration (the credentials Secret must be in the Issuer's
namespace and `credentialsSecret.namespace` must match it):

```yaml
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: letsencrypt
  namespace: issuer-namespace
spec:
  acme:
    email: user@example.com
    server: https://acme-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: letsencrypt-account-key
    solvers:
      - dns01:
          webhook:
            groupName: acme.github.ru13en.123domain.eu
            solverName: "123domain"
            config:
              apiSecretKeyRef:
                name: ns123domain-api-secret
```

The API key and API secret are sent in the API authorization header. The
`ResolvedZone` must be a domain managed by the API account. The solver derives
the API `{toplevel}/{name}` route and relative record name from the resolved
zone and FQDN, and sends TXT values in the record `text` attribute.

## Releases

Pushing a `vX.Y.Z` tag runs the release workflow. It publishes Linux `amd64`
and `arm64` binaries with checksums to a GitHub Release, pushes the container
image to `ghcr.io/ru13en/cert-manager-webhook-123domain` with the version and
`latest` tags, and publishes the Helm chart to
`oci://ghcr.io/ru13en/charts/cert-manager-webhook-123domain`.

Install a published chart by specifying its release version:

```sh
helm upgrade --install cert-manager-webhook-123domain \
  oci://ghcr.io/ru13en/charts/cert-manager-webhook-123domain \
  --version 1.0.0 \
  --namespace cert-manager
```

GitHub Actions needs permission to publish packages. GHCR packages may be
created as private; make the container image and Helm chart public in the
repository's package settings if users should be able to pull them without
authentication.

## Tests

The API client and solver have unit tests using a mock HTTP server:

```bash
go test ./...
```

Running cert-manager's DNS01 conformance suite requires a real domain managed
by the configured 123domain account and credentials accessible to the webhook.
