package main

import (
	"os"
	"testing"
	logf "github.com/jetstack/cert-manager/pkg/logs"
	"github.com/jetstack/cert-manager/test/acme/dns"
	testserver "github.com/jetstack/cert-manager/test/acme/dns/server"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
	kubeBuilderBinPath = "./_out/kubebuilder/bin"
	rfc2136TestDomain      = "123456789.www.example.com"
	rfc2136TestKeyAuth     = "123d=="
	rfc2136TestValue       = "Now36o-3BmlB623-0c1qCIUmgWVVmDJb88KGl24pqpo"
	rfc2136TestFqdn        = "_acme-challenge.123456789.www.example.com."
	rfc2136TestZone        = "example.com."
	rfc2136TestTsigKeyName = "example.com."
	rfc2136TestTTL         = 60
	rfc2136TestTsigSecret  = "IwBTJx9wrDp4Y1RyC3H0gA=="
)

func TestRunsSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.

	ctx := logf.NewContext(nil, nil, t.Name())
	server := &testserver.BasicServer{
		Zones:         []string{rfc2136TestZone},
		EnableTSIG:    true,
		TSIGZone:      rfc2136TestZone,
		TSIGKeyName:   rfc2136TestTsigKeyName,
		TSIGKeySecret: rfc2136TestTsigSecret,
	}
	if err := server.Run(ctx); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer server.Shutdown()

	fixture := dns.NewFixture(&customDNSProviderSolver{},
	    dns.SetBinariesPath(kubeBuilderBinPath),
		dns.SetResolvedZone(zone),
		dns.SetDNSServer(server.ListenAddr()),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath("testdata/my-custom-solver"),
	)

	fixture.RunConformance(t)
}
