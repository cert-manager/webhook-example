package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/JenswBE/cert-manager-webhook-desec/desec"
	"github.com/cert-manager/cert-manager/pkg/issuer/acme/dns/util"
	acmetest "github.com/cert-manager/cert-manager/test/acme"
	"github.com/stretchr/testify/assert"
)

func TestRunsSuite(t *testing.T) {
	// Given
	rrsets := make(desec.RRSets, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Token dummy", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		switch r.URL.Path {
		case "/domains/":
			assert.Equal(t, "GET", r.Method)
			_, err := w.Write([]byte(`[{"name": "example.com", "minimum_ttl": 60}]`))
			assert.NoError(t, err)
		case "/domains/example.com/rrsets/":
			switch r.Method {
			case "GET":
				body, err := json.Marshal(rrsets)
				assert.NoError(t, err)
				_, err = w.Write(body)
				assert.NoError(t, err)
			case "PUT":
				body, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.NoError(t, json.Unmarshal(body, &rrsets))
				w.WriteHeader(200)
				_, err = w.Write(body)
				assert.NoError(t, err)
			default:
				t.Fail()
			}
		default:
			t.Fail()
		}
	}))
	defer server.Close()

	util.PreCheckDNS = func(ctx context.Context, fqdn, value string, nameservers []string, useAuthoritative bool) (bool, error) {
		return slices.ContainsFunc(rrsets, func(rrset desec.RRSet) bool {
			return rrset.Type == "TXT" && slices.Contains(rrset.Records, fmt.Sprintf(`"%s"`, value))
		}), nil
	}

	fixture := acmetest.NewFixture(
		&deSECDNSProviderSolver{BaseUrl: server.URL},
		acmetest.SetResolvedZone("example.com."),
		acmetest.SetAllowAmbientCredentials(false),
		acmetest.SetManifestPath("testdata/desec"),
	)
	// need to uncomment and  RunConformance delete runBasic and runExtended once https://github.com/cert-manager/cert-manager/pull/4835 is merged
	// fixture.RunConformance(t)
	fixture.RunBasic(t)
	fixture.RunExtended(t)
}
