package main

import (
	"os"
	"testing"

	"github.com/jetstack/cert-manager/test/acme/dns"

	"github.com/cert-manager/webhook-example/example"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
)

func TestRunsSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.
	//

	// Uncomment the below fixture when implementing your custom DNS provider
	//fixture := dns.NewFixture(&customDNSProviderSolver{},
	//	dns.SetResolvedZone(zone),
	//	dns.SetAllowAmbientCredentials(false),
	//	dns.SetManifestPath("testdata/my-custom-solver"),
	//	dns.SetBinariesPath("_test/kubebuilder/bin"),
	//)
	os.Setenv("TEST_ASSET_ETCD", "_test/kubebuilder/bin/etcd")
	os.Setenv("TEST_ASSET_KUBE_APISERVER", "_test/kubebuilder/bin/kube-apiserver")
	defer os.Unsetenv("TEST_ASSET_ETCD")
	defer os.Unsetenv("TEST_ASSET_KUBE_APISERVER")
	solver := example.New("59351")
	fixture := dns.NewFixture(solver,
		dns.SetResolvedZone("example.com."),
		dns.SetManifestPath("testdata/my-custom-solver"),
		dns.SetDNSServer("127.0.0.1:59351"),
		dns.SetUseAuthoritative(false),
	)

	fixture.RunBasic(t)
	fixture.RunExtended(t)
}
