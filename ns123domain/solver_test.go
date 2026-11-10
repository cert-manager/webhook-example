package ns123domain

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	acme "github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"
)

func TestSolverName(t *testing.T) {
	if got := New().Name(); got != "123domain" {
		t.Errorf("solver name = %q, want 123domain", got)
	}
}

func TestConfigReferencesCredentialSecretFixture(t *testing.T) {
	rawConfig, err := os.ReadFile(filepath.Join("..", "testdata", "123domain", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(&extapi.JSON{Raw: rawConfig})
	if err != nil {
		t.Fatal(err)
	}

	rawSecret, err := os.ReadFile(filepath.Join("..", "testdata", "123domain", "secret.yaml.example"))
	if err != nil {
		t.Fatal(err)
	}
	var secret struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		StringData map[string]string `json:"stringData"`
	}
	if err := yaml.Unmarshal(rawSecret, &secret); err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.K8S123DomainAPISecretRef.Name, secret.Metadata.Name; got != want {
		t.Errorf("config references Secret %q, fixture Secret is %q", got, want)
	}
	for _, key := range []string{"api-key", "api-secret"} {
		if value := secret.StringData[key]; value == "" {
			t.Errorf("credential Secret fixture has no %q value", key)
		}
	}
}

func TestPresentReadsCredentialsFromReferencedSecret(t *testing.T) {
	rawConfig, err := os.ReadFile(filepath.Join("..", "testdata", "123domain", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	rawSecret, err := os.ReadFile(filepath.Join("..", "testdata", "123domain", "secret.yaml.example"))
	if err != nil {
		t.Fatal(err)
	}
	var secretFixture struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		StringData map[string]string `json:"stringData"`
	}
	if err := yaml.Unmarshal(rawSecret, &secretFixture); err != nil {
		t.Fatal(err)
	}
	secretFixture.StringData["api-key"] = "test-key"
	secretFixture.StringData["api-secret"] = "test-secret"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretFixture.Metadata.Name, Namespace: "issuer-ns"},
		Data:       make(map[string][]byte, len(secretFixture.StringData)),
	}
	for key, value := range secretFixture.StringData {
		secret.Data[key] = []byte(value)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantAuthorization := "Bearer " + "test-key" + ":" + "test-secret"
		if got := r.Header.Get("Authorization"); got != wantAuthorization {
			t.Errorf("Authorization header = %q, want %q", got, wantAuthorization)
		}
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`[]`))
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	s := &solver123Domain{
		client:     fake.NewSimpleClientset(secret),
		httpClient: server.Client(),
		baseURL:    server.URL + "/api",
	}
	challenge := &acme.ChallengeRequest{
		ResourceNamespace: "issuer-ns",
		ResolvedZone:      "example.com.",
		ResolvedFQDN:      "_acme-challenge.example.com.",
		Key:               "challenge-value",
		Config:            &extapi.JSON{Raw: rawConfig},
	}

	if err := s.Present(challenge); err != nil {
		t.Fatal(err)
	}
}
