package main

import (
	"os"
	"testing"

	acmetest "github.com/cert-manager/cert-manager/test/acme"

	"github.com/proton11/cert-manager-desec-webhook/solver"
)

func TestRunsSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.
	//

	// Uncomment the below fixture when implementing your custom DNS provider
	//fixture := acmetest.NewFixture(&customDNSProviderSolver{},
	//	acmetest.SetResolvedZone(zone),
	//	acmetest.SetAllowAmbientCredentials(false),
	//	acmetest.SetManifestPath("testdata/my-custom-solver"),
	//	acmetest.SetBinariesPath("_test/kubebuilder/bin"),
	//)
	desecSolver := &solver.DeSECDNSProviderSolver{}
	zoneName := os.Getenv("TEST_ZONE_NAME")
	if zoneName == "" {
		t.Skip("TEST_ZONE_NAME not set")
	}
	fixture := acmetest.NewFixture(desecSolver,
		acmetest.SetResolvedZone(zoneName),
		acmetest.SetManifestPath("testdata/desec"),
		acmetest.SetDNSServer("127.0.0.1:59351"),
		acmetest.SetUseAuthoritative(false),
	)
	// need to uncomment and  RunConformance delete runBasic and runExtended once https://github.com/cert-manager/cert-manager/pull/4835 is merged
	// fixture.RunConformance(t)
	fixture.RunBasic(t)
	fixture.RunExtended(t)
}
