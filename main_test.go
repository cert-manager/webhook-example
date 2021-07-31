package main

import (
	"os"
	"testing"

	"github.com/jetstack/cert-manager/test/acme/dns"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
	fqdn string
)

func TestRunsSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.
	//
	fqdn = "_acme-challenge.test." + zone

	// Uncomment the below fixture when implementing your custom DNS provider
	fixture := dns.NewFixture(&pdnsDNSProviderSolver{},
		dns.SetResolvedZone(zone),
		dns.SetResolvedFQDN(fqdn),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath("testdata/pdns"),
		dns.SetBinariesPath("_test/kubebuilder/bin"),
	)

	// solver := example.New("59351")
	// fixture := dns.NewFixture(&pdnsDNSProviderSolver{},
	// 	dns.SetResolvedZone("example.com."),
	// 	dns.SetManifestPath("testdata/pdns"),
	// 	dns.SetBinariesPath("_test/kubebuilder/bin"),
	// 	dns.SetDNSServer("127.0.0.1:59351"),
	// 	dns.SetUseAuthoritative(false),
	// )

	fixture.RunConformance(t)
}