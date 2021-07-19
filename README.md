### Deployment

Deploy the custom pdns apiextenion using the helm chart in depploy.

This is how i deployed it.
```
oc project cert-manager
oc apply -f rendered-manifest.yaml
```

### Example Issuer using the staging letsencypt api. 

```
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: dns-acme-issuer
spec:
  acme:
    email: user@example.com
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: acme-account-secret
    solvers:
    - dns01:
        webhook:
          groupName: acme.powerdns.com
          solverName: powerdns
          config:
            server: "http://powerdnsserverurl:80"
            apikey: supersecret
```


