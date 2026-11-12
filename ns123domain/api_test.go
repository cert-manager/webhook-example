package ns123domain

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPresentTXTCreatesRecord(t *testing.T) {
	var calls []string
	getCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAPIRequest(t, r)
		if got, want := r.URL.Path, "/api/v1/domains/com/example/dns"; got != want {
			t.Errorf("request path = %q, want %q", got, want)
		}
		calls = append(calls, r.Method)

		switch r.Method {
		case http.MethodGet:
			getCalls++
			if getCalls == 1 {
				writeAPIFixture(t, w, "dns-records-empty.json")
			} else {
				writeAPIFixture(t, w, "dns-records.json")
			}
		case http.MethodPost:
			if got, want := r.Header.Get("Content-Type"), "application/json"; got != want {
				t.Errorf("Content-Type header = %q, want %q", got, want)
			}
			var record dnsRecord
			if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
				t.Errorf("decoding created record: %v", err)
			}
			var want dnsRecord
			if err := json.Unmarshal(readAPIFixture(t, "dns-record-create.json"), &want); err != nil {
				t.Fatalf("decoding expected record fixture: %v", err)
			}
			if !reflect.DeepEqual(record, want) {
				t.Errorf("created record = %#v, want %#v", record, want)
			}
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	client := newTestDNSAPIClient(server.URL)
	if err := client.presentTXT("example.com.", "_acme-challenge.example.com.", "challenge-value"); err != nil {
		t.Fatal(err)
	}
	if want := []string{http.MethodGet, http.MethodPost, http.MethodGet}; !reflect.DeepEqual(calls, want) {
		t.Errorf("methods = %v, want %v", calls, want)
	}
}

func TestPresentTXTIsIdempotent(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
		}
		assertAPIRequest(t, r)
		writeAPIFixture(t, w, "dns-records.json")
	}))
	defer server.Close()

	if err := newTestDNSAPIClient(server.URL).presentTXT("example.com.", "_acme-challenge.example.com.", "challenge-value"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("request count = %d, want 1", calls)
	}
}

func TestPresentTXTRejectsUnconfirmedCreation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeAPIFixture(t, w, "dns-records-empty.json")
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"name":"_acme-challenge","type":"TXT","ttl":60,"attributes":[{"key":"value","value":"challenge-value"}]}]`))
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	err := newTestDNSAPIClient(server.URL).presentTXT("example.com.", "_acme-challenge.example.com.", "challenge-value")
	if err == nil {
		t.Fatal("expected creation without a returned matching record to fail")
	}
}

func TestIsTXTRecordIgnoresDisabledRecords(t *testing.T) {
	record := dnsRecord{
		Name:     "_acme-challenge",
		Type:     "TXT",
		Disabled: true,
		Attributes: []dnsRecordAttribute{
			{Key: "value", Value: "challenge-value"},
		},
	}
	if isTXTRecord(record, "_acme-challenge", "challenge-value") {
		t.Fatal("disabled TXT record must not count as a presented challenge")
	}
}

func TestCleanTXTDeletesOnlyMatchingChallenge(t *testing.T) {
	var deleted []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			assertAPIRequest(t, r)
			writeAPIFixture(t, w, "dns-records.json")
		case http.MethodDelete:
			assertAPIRequest(t, r)
			deleted = append(deleted, r.Method+" "+r.URL.Path)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	if err := newTestDNSAPIClient(server.URL).cleanTXT("example.com.", "_acme-challenge.example.com.", "challenge-value"); err != nil {
		t.Fatal(err)
	}
	want := []string{"DELETE /api/v1/domains/com/example/dns/sQ7LCJIXSTWkBEsB"}
	if !reflect.DeepEqual(deleted, want) {
		t.Errorf("deleted records = %v, want %v", deleted, want)
	}
}

func TestDNSRecordFixtureMatchesSwaggerExamples(t *testing.T) {
	var records []dnsRecord
	if err := json.Unmarshal(readAPIFixture(t, "dns-records.json"), &records); err != nil {
		t.Fatalf("decoding DNS records fixture: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("DNS records fixture has %d records, want 3", len(records))
	}
	aRecord := records[0]
	if aRecord.ID != "xTbfgvzBXfIqBCQdzspJaA" ||
		aRecord.Name != "www" ||
		aRecord.Type != "A" ||
		aRecord.TTL != 86400 ||
		aRecord.Comment != "Our webserver" ||
		!aRecord.Disabled ||
		aRecord.UserData != "12345" {
		t.Errorf("Swagger A record example decoded as %#v", aRecord)
	}
	if len(records[0].Attributes) != 1 || records[0].Attributes[0] != (dnsRecordAttribute{Key: "ip", Value: "127.0.0.1"}) {
		t.Errorf("Swagger A record attribute decoded as %#v", records[0].Attributes)
	}
	if !isTXTRecord(records[1], "_acme-challenge", "challenge-value") {
		t.Errorf("Swagger-ID TXT challenge fixture not recognized: %#v", records[1])
	}
}

func TestRecordLocation(t *testing.T) {
	domain, name, err := recordLocation("example.co.uk.", "_acme-challenge.example.co.uk.")
	if err != nil {
		t.Fatal(err)
	}
	if domain.name != "example" || domain.toplevel != "co.uk" || name != "_acme-challenge" {
		t.Errorf("record location = %#v, %q", domain, name)
	}

	if _, _, err := recordLocation("example.com.", "not-example.com."); err == nil {
		t.Fatal("expected an FQDN outside the zone to fail")
	}
}

func assertAPIRequest(t *testing.T, r *http.Request) {
	t.Helper()
	if got, want := r.Header.Get("Authorization"), "Bearer test-key:test-secret"; got != want {
		t.Errorf("Authorization header = %q, want %q", got, want)
	}
	if got, want := r.Header.Get("Accept"), "application/json"; got != want {
		t.Errorf("Accept header = %q, want %q", got, want)
	}
}

func writeAPIFixture(t *testing.T, w http.ResponseWriter, name string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(readAPIFixture(t, name)); err != nil {
		t.Errorf("writing API fixture %q: %v", name, err)
	}
}

func readAPIFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "testdata", "123domain", "api", name))
	if err != nil {
		t.Fatalf("reading API fixture %q: %v", name, err)
	}
	return data
}

func newTestDNSAPIClient(baseURL string) *dnsAPIClient {
	return &dnsAPIClient{
		baseURL:       baseURL + "/api",
		authorization: "Bearer " + "test-key" + ":" + "test-secret",
		httpClient:    http.DefaultClient,
	}
}
