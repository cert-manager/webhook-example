package vkcloud

import (
	"context"
	"fmt"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/gophercloud/gophercloud"
	"github.com/pkg/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func NewSolver() webhook.Solver {
	return &Solver{}
}

// Solver implements the provider-specific logic needed to
// 'present' an ACME challenge TXT record for your own DNS provider.
// To do so, it must implement the `github.com/jetstack/cert-manager/pkg/acme/webhook.Solver`
// interface.
type Solver struct {
	client *kubernetes.Clientset
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (s *Solver) Name() string {
	return "vkcloud"
}

func (s *Solver) Present(ch *v1alpha1.ChallengeRequest) error {
	klog.Infof("Presenting txt record: %v %v", ch.ResolvedFQDN, ch.ResolvedZone)
	client, err := s.newClientFromChallenge(ch)
	if err != nil {
		klog.Errorf("New client from challenge error: %v", err)
		return err
	}

	zone, err := client.GetHostedZone(ch.ResolvedZone)
	if err != nil {
		klog.Errorf("Get hosted zone %v error: %v", ch.ResolvedZone, err)
		return err
	}

	name, err := dns01.ExtractSubDomain(ch.ResolvedFQDN, zone.Zone)
	if err != nil {
		klog.Errorf("Get subdomain %v error: %v", ch.ResolvedFQDN, err)
		return err
	}

	records, err := client.ListTXTRecords(zone.UUID)
	if err != nil {
		klog.Errorf("List TXT records %v error: %v", zone.Zone, err)
		return err
	}

	for _, record := range records {
		if record.Name == name && record.Content == ch.Key {
			klog.Infof("The DNSRecord %v%v - %v is already present, nothing to do",
				name,
				ch.ResolvedZone,
				ch.Key,
			)
			return nil
		}
	}

	if err := client.CreateTXTRecord(zone.UUID, &DNSTXTRecord{
		Name:    name,
		Content: ch.Key,
		TTL:     60,
	}); err != nil {
		klog.Errorf("Add txt record %q error: %v", ch.ResolvedFQDN, err)
		return err
	}

	klog.Infof("Presented txt record %v", ch.ResolvedFQDN)
	return nil
}

func (s *Solver) newClientFromChallenge(ch *v1alpha1.ChallengeRequest) (*Client, error) {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return nil, err
	}

	klog.Infof("Decoded config: %v", cfg)

	cred, err := s.getCredential(&cfg, ch.ResourceNamespace)
	if err != nil {
		return nil, fmt.Errorf("get credential error: %v", err)
	}

	client, err := NewClient(cfg.DNSEndpoint, *cred)
	if err != nil {
		return nil, fmt.Errorf("new dns client error: %v", err)
	}

	return client, nil
}

func (s *Solver) getCredential(cfg *Config, namespace string) (*gophercloud.AuthOptions, error) {
	username, err := s.getSecretData(cfg.UsernameSecretRef, namespace)
	if err != nil {
		return nil, err
	}

	password, err := s.getSecretData(cfg.PasswordSecretRef, namespace)
	if err != nil {
		return nil, err
	}

	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: cfg.IdentityEndpoint,
		DomainName:       cfg.UserDomainName,
		Username:         string(username),
		Password:         string(password),
		TenantID:         cfg.ProjectID,
	}

	return &authOpts, nil
}

func (s *Solver) getSecretData(selector cmmetav1.SecretKeySelector, namespace string) ([]byte, error) {
	secret, err := s.client.CoreV1().Secrets(namespace).Get(context.TODO(), selector.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load secret %q", namespace+"/"+selector.Name)
	}

	if data, ok := secret.Data[selector.Key]; ok {
		return data, nil
	}

	return nil, errors.Errorf("no key %q in secret %q", selector.Key, namespace+"/"+selector.Name)
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (s *Solver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	klog.Infof("Cleaning up txt record: %v %v", ch.ResolvedFQDN, ch.ResolvedZone)
	client, err := s.newClientFromChallenge(ch)
	if err != nil {
		klog.Errorf("New client from challenge error: %v", err)
		return err
	}

	zone, err := client.GetHostedZone(ch.ResolvedZone)
	if err != nil {
		klog.Errorf("Get hosted zone %v error: %v", ch.ResolvedZone, err)
		return err
	}

	name, err := dns01.ExtractSubDomain(ch.ResolvedFQDN, zone.Zone)
	if err != nil {
		klog.Errorf("Get subdomain %v error: %v", ch.ResolvedFQDN, err)
		return err
	}

	records, err := client.ListTXTRecords(zone.UUID)
	if err != nil {
		klog.Errorf("Get text record %v.%v error: %v", ch.ResolvedFQDN, zone.UUID, err)
		return err
	}

	for _, record := range records {
		if record.Name == name && record.Content == ch.Key {
			if err := client.DeleteTXTRecord(zone.UUID, record.UUID); err != nil {
				klog.Errorf("Delete domain record %v error: %v", ch.ResolvedFQDN, err)
				return err
			}
			klog.Infof("Cleaned up txt record: %v %v", ch.ResolvedFQDN, ch.ResolvedZone)
		}
	}
	return nil
}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initialising
// connections or warming up caches.
//
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
//
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (s *Solver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	s.client = cl
	return nil
}
