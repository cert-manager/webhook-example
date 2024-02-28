package main

import (
	"webhook-vkcloud/vkcloud"

	"os"
	"testing"

	acmetest "github.com/cert-manager/cert-manager/test/acme"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
)

func TestRunsSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.
	fixture := acmetest.NewFixture(vkcloud.NewSolver(),
		acmetest.SetResolvedZone(zone),
		acmetest.SetAllowAmbientCredentials(false),
		acmetest.SetManifestPath("testdata/vkcloud-solver"),
		//	acmetest.SetBinariesPath("_test/kubebuilder/bin"),
	)
	fixture.RunConformance(t)
}
