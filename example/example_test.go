package example

import (
	"crypto/rand"
	acme "github.com/jetstack/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
)

func TestExampleSolver_Name(t *testing.T) {
	port, _ := rand.Int(rand.Reader, big.NewInt(50000))
	port = port.Add(port, big.NewInt(15534))
	solver := New(port.String())
	assert.Equal(t, "example", solver.Name())
}

func TestExampleSolver_Initialize(t *testing.T) {
	port, _ := rand.Int(rand.Reader, big.NewInt(50000))
	port = port.Add(port, big.NewInt(15534))
	solver := New(port.String())
	done := make(chan struct{})
	err := solver.Initialize(nil, done)
	assert.NoError(t, err, "Expected Initialize not to error")
	close(done)
}

func TestExampleSolver_Present_Cleanup(t *testing.T) {
	port, _ := rand.Int(rand.Reader, big.NewInt(50000))
	port = port.Add(port, big.NewInt(15534))
	solver := New(port.String())
	done := make(chan struct{})
	err := solver.Initialize(nil, done)
	assert.NoError(t, err, "Expected Initialize not to error")

	validTestData := []struct {
		hostname string
		record   string
	}{
		{"test1.example.com.", "testkey1"},
		{"test2.example.com.", "testkey2"},
		{"test3.example.com.", "testkey3"},
	}
	for _, test := range validTestData {
		err := solver.Present(&acme.ChallengeRequest{
			Action:       acme.ChallengeActionPresent,
			Type:         "dns-01",
			ResolvedFQDN: test.hostname,
			Key:          test.record,
		})
		assert.NoError(t, err, "Unexpected error while presenting %v", t)
	}

	// Resolve test data
	for _, test := range validTestData {
		msg := new(dns.Msg)
		msg.Id = dns.Id()
		msg.RecursionDesired = true
		msg.Question = make([]dns.Question, 1)
		msg.Question[0] = dns.Question{dns.Fqdn(test.hostname), dns.TypeTXT, dns.ClassINET}
		in, err := dns.Exchange(msg, "127.0.0.1:"+port.String())

		assert.NoError(t, err, "Presented record %s not resolvable", test.hostname)
		assert.Len(t, in.Answer, 1, "RR response is of incorrect length")
		assert.Equal(t, []string{test.record}, in.Answer[0].(*dns.TXT).Txt, "TXT record returned did not match presented record")
	}

	// Cleanup test data
	for _, test := range validTestData {
		err := solver.CleanUp(&acme.ChallengeRequest{
			Action:       acme.ChallengeActionCleanUp,
			Type:         "dns-01",
			ResolvedFQDN: test.hostname,
			Key:          test.record,
		})
		assert.NoError(t, err, "Unexpected error while cleaning up %v", t)
	}

	// Resolve test data
	for _, test := range validTestData {
		msg := new(dns.Msg)
		msg.Id = dns.Id()
		msg.RecursionDesired = true
		msg.Question = make([]dns.Question, 1)
		msg.Question[0] = dns.Question{dns.Fqdn(test.hostname), dns.TypeTXT, dns.ClassINET}
		in, err := dns.Exchange(msg, "127.0.0.1:"+port.String())

		assert.NoError(t, err, "Presented record %s not resolvable", test.hostname)
		assert.Len(t, in.Answer, 0, "RR response is of incorrect length")
		assert.Equal(t, dns.RcodeNameError, in.Rcode, "Expexted NXDOMAIN")
	}

	close(done)
}
