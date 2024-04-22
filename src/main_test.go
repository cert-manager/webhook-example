package main

import (
	"os"
	"testing"

	dns "github.com/cert-manager/cert-manager/test/acme"
)

var (
	zone         = os.Getenv("TEST_ZONE_NAME")
	testdata_dir = "../testdata"
)

func TestRunsSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.

	fixture := dns.NewFixture(&dnsimpleDNSProviderSolver{},
		dns.SetResolvedZone(zone),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath(testdata_dir),
		dns.SetDNSName("puzzle.beer"),
		dns.SetUseAuthoritative(false),
		dns.SetDNSServer("ns1.dnsimple.com:53"),
	)

	fixture.RunConformance(t)
}
