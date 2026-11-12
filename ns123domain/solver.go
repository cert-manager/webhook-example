package ns123domain

import (
	"context"
	"fmt"
	"log/slog"
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
	slog.Debug("solver Present started", "zone", ch.ResolvedZone, "fqdn", ch.ResolvedFQDN, "namespace", ch.ResourceNamespace)
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		slog.Debug("solver Present failed to load config", "error", err)
		return err
	}
	api, err := s.apiClient(ch, cfg)
	if err != nil {
		slog.Debug("solver Present failed to initialize DNS API client", "error", err)
		return err
	}
	if err := api.presentTXT(ch.ResolvedZone, ch.ResolvedFQDN, ch.Key); err != nil {
		slog.Debug("solver Present failed to present TXT challenge", "zone", ch.ResolvedZone, "fqdn", ch.ResolvedFQDN)
		return err
	}
	slog.Debug("solver Present completed", "zone", ch.ResolvedZone, "fqdn", ch.ResolvedFQDN)
	return nil
}

// CleanUp removes only the TXT record belonging to this challenge.
func (s *solver123Domain) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	slog.Debug("solver CleanUp started", "zone", ch.ResolvedZone, "fqdn", ch.ResolvedFQDN, "namespace", ch.ResourceNamespace)
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		slog.Debug("solver CleanUp failed to load config", "error", err)
		return err
	}
	api, err := s.apiClient(ch, cfg)
	if err != nil {
		slog.Debug("solver CleanUp failed to initialize DNS API client", "error", err)
		return err
	}
	if err := api.cleanTXT(ch.ResolvedZone, ch.ResolvedFQDN, ch.Key); err != nil {
		slog.Debug("solver CleanUp failed to remove TXT challenge", "zone", ch.ResolvedZone, "fqdn", ch.ResolvedFQDN)
		return err
	}
	slog.Debug("solver CleanUp completed", "zone", ch.ResolvedZone, "fqdn", ch.ResolvedFQDN)
	return nil
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
	slog.Debug("reading 123domain credentials Secret", "namespace", ch.ResourceNamespace, "secret", cfg.K8S123DomainAPISecretRef.Name)
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
	slog.Debug("loaded 123domain credentials", "namespace", ch.ResourceNamespace, "secret", cfg.K8S123DomainAPISecretRef.Name, "hasAPIKey", true, "hasAPISecret", true)

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
	slog.Debug("initializing Kubernetes client for 123domain solver")
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		slog.Debug("failed to initialize Kubernetes client for 123domain solver", "error", err)
		return err
	}
	s.client = cl
	slog.Debug("initialized Kubernetes client for 123domain solver")
	return nil
}
