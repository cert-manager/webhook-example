package main

import (
	"os"
	"testing"
	"time"

	dns "github.com/cert-manager/cert-manager/test/acme"
	"github.com/stretchr/testify/assert"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
)

func TestRunsSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.

	pollTime, _ := time.ParseDuration("10s")
	timeOut, _ := time.ParseDuration("5m")

	fixture := dns.NewFixture(&gcoreDNSProviderSolver{},
		dns.SetResolvedZone(zone),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath("testdata/gcore"),

		// Disable the extended test to create several records for the same Record DNS Name
		dns.SetStrict(false),
		// Increase the poll interval to 10s
		dns.SetPollInterval(pollTime),
		// Increase the limit from 2 min to 5 min
		dns.SetPropagationLimit(timeOut),
	)

	fixture.RunConformance(t)

}

func Test_extractAllZones(t *testing.T) {
	testCases := []struct {
		desc     string
		fqdn     string
		expected []string
	}{
		{
			desc:     "success",
			fqdn:     "_acme-challenge.my.test.domain.com.",
			expected: []string{"my.test.domain.com", "test.domain.com", "domain.com"},
		},
		{
			desc: "empty",
			fqdn: "_acme-challenge.com.",
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			got := extractAllZones(test.fqdn)
			assert.Equal(t, test.expected, got)
		})
	}
}
