package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"testing"

	"time"

	//"gitlab.com/smueller18/cert-manager-webhook-inwx/test"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	"github.com/gstore/cert-manager-webhook-dynu/dynuclient"
	guntest "github.com/gstore/cert-manager-webhook-dynu/test"
	test "github.com/gstore/cert-manager-webhook-dynu/test"
	"github.com/stretchr/testify/assert"

	logf "github.com/jetstack/cert-manager/pkg/logs"
	"github.com/jetstack/cert-manager/test/acme/dns"
	"github.com/jetstack/cert-manager/test/acme/dns/server"
)

var (
	zone               = os.Getenv("TEST_ZONE_NAME")
	kubeBuilderBinPath = "./_out/kubebuilder/bin"
	fqdn               string
)

func TestRunsSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.
	dnsResp := dynuclient.DNSResponse{
		StatusCode: 200,
		ID:         98765,
		DomainID:   123456,
		DomainName: "domainName",
		NodeName:   "nodeName",
		Hostname:   "hostName",
		RecordType: "TXT",
		TTL:        90,
		State:      true,
		Content:    "content",
		UpdatedOn:  "2020-10-29T23:00",
	}
	dnsResponse, err := json.Marshal(dnsResp)
	if err != nil {
		fmt.Printf("error decoding solver config: %v", err)
	}
	scenarios := []struct {
		scenario string
		expected string
		actual   string
	}{
		{
			scenario: "DNS Record create",
			expected: "/v2/dns/123456/record",
		},
		{
			scenario: "DNS Record delete",
			expected: "/v2/dns/123456/record/98765",
		},
	}
	httpCall := 0
	fqdn = "cert-manager-dns01-tests-with-secret." + zone
	testHandlerFunc := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, scenarios[httpCall].expected, req.URL.String(), scenarios[httpCall].scenario+" failed")
		httpCall++
		w.Write([]byte(dnsResponse))
	})

	client := &guntest.Testclient{}
	httpClient, teardown := client.TestingHTTPClient(testHandlerFunc)
	defer teardown()

	fqdn = "cert-manager-dns01-tests." + zone
	ctx := logf.NewContext(nil, nil, t.Name())
	txtRecs := map[string][][]string{
		fqdn: {
			{},
			{},
			{"123d=="},
			{"123d=="},
		},
	}
	srv := &server.BasicServer{
		Handler: &test.DNSHandler{
			Log:        logf.FromContext(ctx, "dnsBasicServerSecret"),
			TxtRecords: txtRecs,
			Zones:      []string{zone},
		},
	}

	if err := srv.Run(ctx); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer srv.Shutdown()
	d, err := ioutil.ReadFile("testdata/config.json")
	if err != nil {
		log.Fatal(err)
	}

	fixture := dns.NewFixture(&customDNSProviderSolver{httpClient: httpClient},
		dns.SetResolvedZone(zone),
		dns.SetResolvedFQDN(fqdn),
		dns.SetAllowAmbientCredentials(false),
		dns.SetBinariesPath(kubeBuilderBinPath),
		dns.SetDNSServer(srv.ListenAddr()),
		dns.SetPropagationLimit(time.Duration(60)*time.Second),
		dns.SetUseAuthoritative(false),
		dns.SetConfig(&extapi.JSON{
			Raw: d,
		}),
	)

	fixture.RunConformance(t)
}
func TestRunSuiteWithSecret(t *testing.T) {
	dnsResp := dynuclient.DNSResponse{
		StatusCode: 200,
		ID:         98765,
		DomainID:   123456,
		DomainName: "domainName",
		NodeName:   "nodeName",
		Hostname:   "hostName",
		RecordType: "TXT",
		TTL:        200,
		State:      true,
		Content:    "content",
		UpdatedOn:  "2020-10-29T23:00",
	}
	dnsResponse, err := json.Marshal(dnsResp)
	if err != nil {
		fmt.Printf("error decoding solver config: %v", err)
	}
	scenarios := []struct {
		scenario string
		expected string
		actual   string
	}{
		{
			scenario: "DNS Record create",
			expected: "/v2/dns/123456/record",
		},
		{
			scenario: "DNS Record delete",
			expected: "/v2/dns/123456/record/98765",
		},
	}
	httpCall := 0
	if os.Getenv("TEST_ZONE_NAME") != "" {
		zone = os.Getenv("TEST_ZONE_NAME")
	}
	fqdn = "cert-manager-dns01-tests-with-secret." + zone
	testHandlerFunc := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, scenarios[httpCall].expected, req.URL.String(), scenarios[httpCall].scenario+" failed")
		httpCall++
		w.Write([]byte(dnsResponse))
	})

	client := &guntest.Testclient{}
	httpClient, teardown := client.TestingHTTPClient(testHandlerFunc)
	defer teardown()

	fqdn = "cert-manager-dns01-tests." + zone
	ctx := logf.NewContext(nil, nil, t.Name())

	txtRecs := map[string][][]string{
		fqdn: {
			{},
			{},
			{"123d=="},
			{"123d=="},
		},
	}

	srv := &server.BasicServer{
		Handler: &test.DNSHandler{
			Log:        logf.FromContext(ctx, "dnsBasicServerSecret"),
			TxtRecords: txtRecs,
			Zones:      []string{zone},
		},
	}

	if err := srv.Run(ctx); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer srv.Shutdown()

	d, err := ioutil.ReadFile("testdata/config.json")
	if err != nil {
		log.Fatal(err)
	}

	fixture := dns.NewFixture(&customDNSProviderSolver{httpClient: httpClient},
		dns.SetResolvedZone(zone),
		dns.SetResolvedFQDN(fqdn),
		dns.SetAllowAmbientCredentials(false),
		dns.SetDNSServer(srv.ListenAddr()),
		dns.SetManifestPath("testdata/secret-dynu-credentials.yaml"),
		dns.SetBinariesPath(kubeBuilderBinPath),
		dns.SetPropagationLimit(time.Duration(90)*time.Second),
		dns.SetUseAuthoritative(false),
		dns.SetConfig(&extapi.JSON{
			Raw: d,
		}),
	)

	fixture.RunConformance(t)
}
