# 123domain solver configuration example

`config.json` is the webhook solver configuration for an Issuer or
ClusterIssuer. Create the referenced Kubernetes Secret in the Issuer namespace
or cert-manager's configured cluster resource namespace. The Secret must have
`api-key` and `api-secret` data entries; see [`secret.yaml.example`](./secret.yaml.example)
for the matching Secret manifest. Replace both placeholder values before
applying it. Do not put credential values in `config.json`; it contains only
the Secret reference.

The chart's `credentialsSecret.name` and `credentialsSecret.namespace` values
must identify that same Secret so the webhook service account can read it.
The config JSON uses the same Secret name in `apiSecretKeyRef`.

The `api/` directory contains DNS record request and response fixtures. Common
DNS record field examples (record ID, name, TTL, comment, disabled flag,
userdata, and `ip` attribute) come from the
[123domain.eu Swagger specification](https://api.123domain.eu/spec/swagger.json);
TXT fixture values model the `value` attribute used when publishing ACME
challenges.
