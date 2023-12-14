package main

import (
	"os"
	"testing"

	acmetest "github.com/cert-manager/cert-manager/test/acme"
)

var (
	zone      = os.Getenv("TEST_ZONE_NAME")
	dnsRecord = os.Getenv("TEST_DNS_RECORD")
)

func TestRunsSuite(t *testing.T) {
	solver := &ibmCloudCisProviderSolver{}
	fixture := acmetest.NewFixture(solver,
		acmetest.SetStrict(true),
		acmetest.SetDNSName(dnsRecord),
		acmetest.SetResolvedZone(zone),
		acmetest.SetManifestPath("testdata/ibm-cloud-cis"),
		acmetest.SetUseAuthoritative(false),
	)
	fixture.RunConformance(t)

}
