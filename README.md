# cert-manager webhook for Namecheap

- This is a Frankenstein version of
  - <https://github.com/kelvie/cert-manager-webhook-namecheap>
  - <https://github.com/Extrality/cert-manager-webhook-namecheap>

- This is as good as any other implementation!
- I just had to find out, that my local `dnsmasq` messed around. When the webhook is trying to find out the zone of the domain, it just got back some local info.
- The workaround (for me) - is to use a public DNS for the cert-manager.
- Idea can be found at [Techno Tim's Repo](https://github.com/techno-tim/launchpad/blob/c18b2c3e3bf9cfa30974ae0e993e5c2fc3c37408/kubernetes/traefik-cert-manager/cert-manager/values.yaml#L9)



# Instructions for use with Let's Encrypt

Thanks to [Addison van den Hoeven](https://github.com/Addyvan), from https://github.com/jetstack/cert-manager/issues/646

Use helm to deploy this into your `cert-manager` namespace:

``` sh
# Make sure you're in the right context:
# kubectl config use-context mycontext

# cert-manager is by default in the cert-manager context
helm install -n cert-manager namecheap-webhook deploy/cert-manager-webhook-namecheap/
```

Create the cluster issuers:

``` sh
helm install --set email=yourname@example.com -n cert-manager letsencrypt-namecheap-issuer deploy/letsencrypt-namecheap-issuer/
```

Get your local public ip: `curl https://ifconfig.co/ip`

Go to namecheap and set up your API key (note that you'll need to whitelist the
public IP of the k8s cluster to use the webhook), and set the secret:

``` yaml
apiVersion: v1
kind: Secret
metadata:
  name: namecheap-credentials
  namespace: cert-manager
type: Opaque
stringData:
  apiKey: my_api_key_from_namecheap
  apiUser: my_username_from_namecheap
  #clientIP: 1.2.3.4 # optional, if your setup can't detect the public IP
```

Now you can create a certificate in staging for testing:

``` yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: wildcard-cert-stage
  namespace: default
spec:
  secretName: wildcard-cert-stage
  commonName: "*.<domain>"
  issuerRef:
    kind: ClusterIssuer
    name: letsencrypt-stage
  dnsNames:
  - "*.<domain>"
```

And now validate that it worked:

``` sh
kubectl get certificates -n default
kubectl describe certificate wildcard-cert-stage
```

And finally, create your production cert, and it'll be ready to use in the
`wildcard-cert-prod` secret.

``` yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: wildcard-cert-prod
  namespace: default
spec:
  secretName: wildcard-cert-prod
  commonName: "*.<domain>"
  issuerRef:
    kind: ClusterIssuer
    name: letsencrypt-prod
  dnsNames:
  - "*.<domain>"
```

TODO: add simple nginx example to test that it works

### Running the test suite

#### Steps

1. Create testdata/namecheap/apikey.yaml and testdata/namecheap/config.json using your credentials.
2. Run `TEST_ZONE_NAME=example.com. make test` . Note that the domain here should be updated to your own
domain name. Also note that this is a full domain name with a `.` at the end.
3. You should see all tests passing.
4. In case the tests fail: set `useSandbox` to false
