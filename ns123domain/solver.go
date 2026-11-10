package ns123domain

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const apiBaseURL = "https://api.123domain.eu/api"

type solver123Domain struct {
	client     kubernetes.Interface
	httpClient *http.Client
	baseURL    string
}

var _ webhook.Solver = (*solver123Domain)(nil)

// New returns a cert-manager DNS solver backed by the 123domain.eu DNS API.
func New() webhook.Solver {
	return &solver123Domain{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    apiBaseURL,
	}
}

func (s *solver123Domain) Name() string {
	return "123domain"
}

func (s *solver123Domain) Present(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}
	api, err := s.apiClient(ch, cfg)
	if err != nil {
		return err
	}
	return api.presentTXT(ch.ResolvedZone, ch.ResolvedFQDN, ch.Key)
}

// CleanUp removes only the TXT record belonging to this challenge.
func (s *solver123Domain) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}
	api, err := s.apiClient(ch, cfg)
	if err != nil {
		return err
	}
	return api.cleanTXT(ch.ResolvedZone, ch.ResolvedFQDN, ch.Key)
}

func (s *solver123Domain) apiClient(ch *v1alpha1.ChallengeRequest, cfg providerConfig) (*dnsAPIClient, error) {
	if s.client == nil {
		return nil, fmt.Errorf("Kubernetes client is not initialized")
	}
	if ch.ResourceNamespace == "" {
		return nil, fmt.Errorf("challenge resource namespace is empty")
	}
	if cfg.K8S123DomainAPISecretRef.Name == "" {
		return nil, fmt.Errorf("apiSecretKeyRef must specify a Secret name")
	}
	secret, err := s.client.CoreV1().Secrets(ch.ResourceNamespace).Get(context.Background(), cfg.K8S123DomainAPISecretRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("reading 123domain credentials Secret %q in namespace %q: %w", cfg.K8S123DomainAPISecretRef.Name, ch.ResourceNamespace, err)
	}
	apiKey, ok := secret.Data["api-key"]
	if !ok || len(apiKey) == 0 {
		return nil, fmt.Errorf("123domain credentials Secret %q in namespace %q has no non-empty key %q", cfg.K8S123DomainAPISecretRef.Name, ch.ResourceNamespace, "api-key")
	}
	apiSecret, ok := secret.Data["api-secret"]
	if !ok || len(apiSecret) == 0 {
		return nil, fmt.Errorf("123domain credentials Secret %q in namespace %q has no non-empty key %q", cfg.K8S123DomainAPISecretRef.Name, ch.ResourceNamespace, "api-secret")
	}

	httpClient := s.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	baseURL := s.baseURL
	if baseURL == "" {
		baseURL = apiBaseURL
	}
	return &dnsAPIClient{
		baseURL:       baseURL,
		authorization: "Bearer " + string(apiKey) + ":" + string(apiSecret),
		httpClient:    httpClient,
	}, nil
}

func (s *solver123Domain) Initialize(kubeClientConfig *rest.Config, _ <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}
	s.client = cl
	return nil
}
