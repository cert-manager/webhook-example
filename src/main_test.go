package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	dns "github.com/cert-manager/cert-manager/test/acme"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	testdata_dir = "../testdata"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
)

func TestRunsSuite(t *testing.T) {
	log.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{})))

	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.

	// Ensure trailing dot
	if !strings.HasSuffix(zone, ".") {
		zone = fmt.Sprintf("%s.", zone)
	}

	fixture := dns.NewFixture(&dnsimpleDNSProviderSolver{},
		dns.SetResolvedZone(zone),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath(testdata_dir),
		dns.SetUseAuthoritative(false),
		dns.SetDNSServer("ns1.dnsimple.com:53"),
		// check against dnsimple nameservers for faster propagation
		dns.SetStrict(true),
	)

	fixture.RunConformance(t)
}
