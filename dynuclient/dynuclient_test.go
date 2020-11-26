package dynuclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	guntest "github.com/gstore/cert-manager-webhook-dynu/test"
	"github.com/stretchr/testify/assert"
)

var i int

// func testingHTTPClient(handler http.Handler) (*http.Client, func()) {
// 	s := httptest.NewTLSServer(handler)

// 	cli := &http.Client{
// 		Transport: &http.Transport{
// 			DialContext: func(_ context.Context, network, _ string) (net.Conn, error) {
// 				return net.Dial(network, s.Listener.Addr().String())
// 			},
// 			TLSClientConfig: &tls.Config{
// 				InsecureSkipVerify: true,
// 			},
// 		},
// 	}

// 	return cli, s.Close
// }
func TestRemoveDNSRecord(t *testing.T) {
	expectedMethod := "DELETE"
	dnsID := "1"
	expectedURL := fmt.Sprintf("/v2/dns/%s/record/12345", dnsID)

	testHandlerFunc := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, expectedURL, req.URL.String(), "Should call %s but called %s", expectedURL, req.URL.String())
		assert.Equal(t, expectedMethod, req.Method, "Should be %s but got %s", expectedMethod, req.Method)

		w.Write([]byte("ok"))
	})
	client := &guntest.Testclient{}
	httpClient, teardown := client.TestingHTTPClient(testHandlerFunc)
	defer teardown()

	dynu := DynuClient{HTTPClient: httpClient, DNSID: dnsID}
	err := dynu.RemoveDNSRecord(12345)
	assert.Nil(t, err, "error returned")
}

func TestCreateDNSRecord(t *testing.T) {
	expectedURL := "/v2/dns/1/record"
	expectedMethod := "POST"
	expectedRecordID := 987654
	domainID := 1

	rec := DNSRecord{
		NodeName:   "asgard",
		RecordType: "TXT",
		TextData:   "some text",
		TTL:        "90",
	}
	dnsResp := DNSResponse{
		StatusCode: 200,
		ID:         expectedRecordID,
		DomainID:   domainID,
		DomainName: "domainName",
		NodeName:   "nodeName",
		Hostname:   "hostName",
		RecordType: "TXT",
		TTL:        90,
		State:      true,
		Content:    "content",
		UpdatedOn:  "2020-10-29T23:00",
		TextData:   "Some text",
	}
	dnsResponse, _ := json.Marshal(dnsResp)
	testHandlerFunc := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, expectedURL, req.URL.String(), "Should call %s but called %s", expectedURL, req.URL.String())
		assert.Equal(t, expectedMethod, req.Method, "Should be %s but got %s", expectedMethod, req.Method)

		w.Write([]byte(dnsResponse))
	})

	client := &guntest.Testclient{}
	httpClient, teardown := client.TestingHTTPClient(testHandlerFunc)
	defer teardown()
	dynu := DynuClient{HTTPClient: httpClient, DNSID: strconv.Itoa(domainID)}
	recordID, err := dynu.CreateDNSRecord(rec)
	if err != nil {
		fmt.Println("an error occured: ", err.Error())
		return
	}

	assert.Equal(t, expectedRecordID, recordID, "RecordID expected %d got %d", expectedRecordID, recordID)
}

func TestAddAndRemoveRecord(t *testing.T) {
	t.Skip("This has been intentionally skipped as it runs a test against the Live API.")
	dnsID := 1
	apiKey := "SomeAPIKey"
	rec := DNSRecord{
		NodeName:   "txt",
		RecordType: "TXT",
		TextData:   "some text",
		TTL:        "300",
		DomainID:   1,
		State:      true,
	}

	d := &DynuClient{DNSID: strconv.Itoa(dnsID), APIKey: apiKey}
	dnsrecordid, err := d.CreateDNSRecord(rec)

	assert.NoError(t, err)
	assert.NotNil(t, dnsrecordid, "DNSRecordID", dnsrecordid)
	t.Logf("CREATED DNSRecordID: %d", dnsrecordid)
	err = d.RemoveDNSRecord(dnsrecordid)
	assert.NoError(t, err)
	t.Logf("Removed DNSRecordID: %d", dnsrecordid)
}
