# CONTRIBUTING

## Install Pre-requisites

1. Install asdf if not already installed. See [asdf's installation instructions here](https://asdf-vm.com/guide/getting-started.html).

1. Install go using [asdf](https://asdf-vm.com/):
   ```bash
   asdf plugin add golang
   asdf install
   ```

## Run Tests

You will need to provide your own IBM Cloud Internet Services instance to run the tests.

Once you have provisioned the instance, update [testdata/ibm-cloud-cis/config.json](./../testdata/ibm-cloud-cis/config.json) with your instance's CRN.

You can run the test suite with:
```bash
IBMCLOUD_API_KEY=<your_api_key> TEST_ZONE_NAME=example.com TEST_DNS_RECORD=test.example.com make test
```
