//go:build e2e

package main

import (
	"os"
	"testing"

	acmetest "github.com/cert-manager/cert-manager/test/acme"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestRunsSuiteE2E(t *testing.T) {
	// Setup logger
	log.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

	// Read domain from env var
	testDomain := os.Getenv("TEST_ZONE_NAME")
	if testDomain == "" {
		t.Fatal("Environment variable TEST_ZONE_NAME must be specified")
	}

	// Create an run fixtures
	fixture := acmetest.NewFixture(
		NewDeSECDNSProviderSolver(),
		acmetest.SetResolvedZone(testDomain),
		acmetest.SetAllowAmbientCredentials(false),
		acmetest.SetManifestPath("testdata/desec_e2e"),
	)
	// need to uncomment and  RunConformance delete runBasic and runExtended once https://github.com/cert-manager/cert-manager/pull/4835 is merged
	// fixture.RunConformance(t)
	fixture.RunBasic(t)
	fixture.RunExtended(t)
}
