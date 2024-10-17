# DNSimple Webhook for cert-manager

A [cert-manager][2] ACME DNS01 solver webhook for [DNSimple][1].


## Pre-requisites

- [cert-manager][2] >= 1.0.0 (The Helm chart uses the new API versions)
- Kubernetes >= 1.17.x
- Helm 3 (otherwise adjust the example below accordingly)


## Quickstart

1. Take note of your DNSimple API token from the account settings in the automation tab.

2. Add the helm repo published under the [Github pages deployment of this repository][4]:
    ```bash
    $ helm repo add certmanager-webhook https://puzzle.github.io/cert-manager-webhook-dnsimple
    ```

3. Install the application, replacing the API token and email placeholders:
    ```bash
    $ helm repo add certmanager-webhook https://puzzle.github.io/cert-manager-webhook-dnsimple
    $ helm install cert-manager-webhook-dnsimple \
        --dry-run \ # remove once you are sure the values are correct
        --namespace cert-manager \
        --set dnsimple.token='<DNSIMPLE_API_TOKEN>' \
        --set clusterIssuer.production.enabled=true \
        --set clusterIssuer.staging.enabled=true \
        --set clusterIssuer.email=<ISSUER_MAIL> \
        certmanager-webhook/cert-manager-webhook-dnsimple
    ```
    Alternatively you can check out this repository and substitute the source of the install command with `./charts/cert-manager-webhook-dnsimple`.

4. Afterwards you can issue a certificate:
    ```bash
    $ cat << EOF | kubectl apply -f -
    apiVersion: cert-manager.io/v1
    kind: Certificate
    metadata:
      name: dnsimple-test
    spec:
      dnsNames:
        - test.example.com
      issuerRef:
        name: cert-manager-webhook-dnsimple-staging
        kind: ClusterIssuer
      secretName: dnsimple-test-tls
    EOF
    ```


## Chart options
The Helm chart accepts the following values:

| name                               | required | description                                     | default value                           |
| ---------------------------------- | -------- | ----------------------------------------------- | --------------------------------------- |
| `dnsimple.token`                   | ✔️       | DNSimple API Token                              | _empty_                                 |
| `dnsimple.accountID`               |          | DNSimple Account ID (required when `dnsimple.token` is a user-token)  | _empty_           |
| `clusterIssuer.email`              |          | LetsEncrypt Admin Email                         | _empty_                                 |
| `clusterIssuer.production.enabled` |          | Create a production `ClusterIssuer`             | `false`                                 |
| `clusterIssuer.staging.enabled`    |          | Create a staging `ClusterIssuer`                | `false`                                 |
| `image.repository`                 | ✔️       | Docker image for the webhook solver             | `ghcr.io/puzzle/cert-manager-webhook-dnsimple` |
| `image.tag`                        | ✔️       | Docker image tag of the solver                  | latest tagged docker build              |
| `image.pullPolicy`                 | ✔️       | Image pull policy of the solver                 | `IfNotPresent`                          |
| `logLevel`                         |          | Set the verbosity of the solver                 | _empty_                                 |
| `useUnprivilegedPort`              |          | Use an unprivileged container-port for the webhook  | `true`                              |
| `groupName`                        | ✔️       | Name of the API group used to register the webhook API service as | `acme.dnsimple.com`                                 |
| `certManager.namespace`            | ✔️       | The namespace cert-manager was installed to     | `cert-manager`                          |
| `certManager.serviceAccountName`   | ✔️       | The service account cert-manager runs under     | `cert-manager`                          |


## Testing
All cert-manager webhooks have to pass the DNS01 provider conformance testing suite.

### Pull requests
Prerequisites for PRs are implemented as  GitHub-actions. All tests should pass before a PR is merged:
- the `cert-manager` conformance suite is run with provided kubebuilder fixtures
- a custom test suite running on a working k8s cluster (using `minikube`) is executed as well

### Local testing
#### Test suite
You can also run tests locally, as specified in the `Makefile`:

1. Set-up `testdata/` according to its [README][3].
    - `dnsimple-token.yaml` should be filled with a valid token (for either the sandbox or production environment)
    - `dnsimple.env` should contain the remaining environment variables (non sensitive)
2. Execute the test suite:
    ```bash
    make test
    ```
#### In-cluster testing
1. Install cert-manager:
    ```bash
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.3/cert-manager.yaml
    ```
2. Install the webhook:
    ```bash
    helm install cert-manager-webhook-dnsimple \
        --namespace cert-manager \
        --set dnsimple.token='<DNSIMPLE TOKEN>' \
        --set clusterIssuer.staging.enabled=true \
        ./charts/cert-manager-webhook-dnsimple
    ```
3. Test away... You can create a sample certificate to ensure the webhook is working correctly:
    ```bash
    kubectl apply -f - <<<EOF
    apiVersion: cert-manager.io/v1
    kind: Certificate
    metadata:
      name: dnsimple-test
    spec:
      dnsNames:
        - test.example.com
      issuerRef:
        name: cert-manager-webhook-dnsimple-staging
        kind: ClusterIssuer
      secretName: dnsimple-test-tls
    EOF
    ```


## Releases
### Docker images
Every push to `master` or on a pull-request triggers the upload of a new docker image to the GitHub Container Registry (this is configured through github actions). These images should **not considered stable** and are tagged with `commit-<hash>`. **We recommend using a specific version tag for production deployments instead.**

Tagged images are considered stable, these are the ones referenced by the default helm values.

### How to tag
Create a new tag and push it to the repository. This will trigger a new container build:
```bash
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```
We recommend the following versioning scheme: `vX.Y.Z` where `X` is the major version, `Y` the minor version and `Z` the patch version.

### Helm releases
Helm charts are only released when significant changes occur. We encourage users to update the underlying image versions on their own. A new release can be  triggered manually under the _actions_ tab and running `helm-release`. This only works if a new version was specified in the `Chart.yaml`. The new release will be appended to the [Github pages deployment][4].


## Contributing
We welcome contributions. Please open an issue or a pull request.



[1]: https://dnsimple.com/
[2]: https://cert-manager.io/docs/installation/kubernetes/
[3]: ./testdata/README.md
[4]: https://puzzle.github.io/cert-manager-webhook-dnsimple
