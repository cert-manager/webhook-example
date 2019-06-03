package main

import (
	cmapi "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	logf "github.com/jetstack/cert-manager/pkg/logs"
	"github.com/jetstack/cert-manager/test/acme/dns"
	testserver "github.com/jetstack/cert-manager/test/acme/dns/server"
	"os"
	"testing"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
	kubeBuilderBinPath = "./_out/kubebuilder/bin"
	rfc2136TestFqdn        = "_acme-challenge.123456789.www.example.com."
	rfc2136TestZone        = "example.com."
	rfc2136TestTsigKeyName = "example.com."
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

	var validConfig = cmapi.ACMEIssuerDNS01ProviderRFC2136{
		Nameserver: server.ListenAddr(),
	}

	fixture := dns.NewFixture(&customDNSProviderSolver{},
	    dns.SetBinariesPath(kubeBuilderBinPath),
		dns.SetResolvedZone(zone),
		dns.SetResolvedFQDN(rfc2136TestFqdn),
		dns.SetConfig(validConfig),
		dns.SetDNSServer(server.ListenAddr()),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath("testdata/my-custom-solver"),
		dns.SetUseAuthoritative(false),
	)

	fixture.RunConformance(t)
}
