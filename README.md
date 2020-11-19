# Cert Manager dynu ACME webhook

Webhook to get a certificate for dynu dns provider

### Running the test suite

All DNS providers **must** run the DNS01 provider conformance testing suite,
else they will have undetermined behaviour when used with cert-manager.

**It is essential that you configure and run the test suite when creating a
DNS01 webhook.**

You can run the test suite with:

```bash
$ TEST_ZONE_NAME=example.com go test .
```
