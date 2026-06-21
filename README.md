# deSEC Webhook for cert-manager

A [cert-manager](https://github.com/cert-manager/cert-manager) webhook to solve an ACME DNS01 challenge using the [deSEC](https://desec.io/) API.

## Prerequisites

A Kubernetes cluster with cert-manager deployed. If you haven't already installed cert-manager, follow the guide [here](https://cert-manager.io/docs/installation/kubernetes/).

## Deployment

### Using Helm

```bash
helm install desec-webhook deploy/desec-webhook \
  --set groupName=acme.yourdomain.com
```

## Usage

### Deploy an API Token Secret

The deSEC API token needs to be placed into a Kubernetes secret. Use `examples/desec-token.yaml` as a starting point. Place your API token into the manifest.

### Deploy an Issuer

An example `ClusterIssuer` is provided in `examples/letsencrypt-staging-issuer.yaml`. It uses the Let's Encrypt staging server. Replace `groupName` with the value you set during deployment.

### Deploy a Certificate

An example certificate manifest is provided in `examples/test-certificate.yaml`.

## Building

```bash
make build
```

## Running the test suite

### Against a mock server

```bash
# Run the tests
make test
```

### Against the deSEC API

```bash
# Copy the example secret file
cp examples/desec-token.yaml testdata/desec_e2e/desec-token.yaml

# Replace <API-Token> with your deSEC API token
editor testdata/desec_e2e/desec-token.yaml

# Set the test zone to run the tests against
export TEST_ZONE_NAME=example.com.

# Run the E2E tests
make test-e2e
```

## AI disclaimer

Copilot was used to do the initial merge of [cert-manager/webhook-example](https://github.com/cert-manager/webhook-example) and [j-be/cert-manager-webhook-desec](https://github.com/j-be/cert-manager-webhook-desec). However, all code was manually reviewed, updated and tested.

## Credits

Main repo structure forked from [cert-manager/webhook-example](https://github.com/cert-manager/webhook-example).

deSEC API client and webhook implementation are based on [j-be/cert-manager-webhook-desec](https://github.com/j-be/cert-manager-webhook-desec).

Both are licensed under Apache-2.0.
