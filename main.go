package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"slices"
	"strings"
	"time"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	logf "github.com/cert-manager/cert-manager/pkg/logs"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	externaldns "sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/lion7/cert-manager-webhook-external-dns/internal/scheme"
	contextutil "github.com/lion7/cert-manager-webhook-external-dns/internal/util/context"
)

var (
	// ProviderName is the presented name of the provider
	ProviderName = "external-dns"

	// GroupName is the Kubernetes group name that will be forwarded to this
	// extension-apiserver.
	GroupName = "cert-manager-webhook.lion7.dev"

	// RequestTimeout is the timeout for each request
	RequestTimeout = time.Second * 5
)

func main() {
	cmd.RunWebhookServer(GroupName,
		&externalDNSProviderSolver{},
	)
}

// externalDNSProviderSolver implements the provider-specific logic needed to
// 'present' an ACME challenge TXT record for your own DNS provider.
// To do so, it must implement the `github.com/cert-manager/cert-manager/pkg/acme/webhook.Solver`
// interface.
type externalDNSProviderSolver struct {
	client client.Client
	ctx    context.Context
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (c *externalDNSProviderSolver) Name() string {
	return ProviderName
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *externalDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	// Create child context that times out
	ctx, cancel := context.WithTimeout(c.ctx, RequestTimeout)
	defer cancel()

	// Fail early if the challenge provided contains bad config
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return fmt.Errorf("could not load solver config: %w", err)
	}

	// All challenges for the same record share one DNSEndpoint object, we
	// just define the metadata here so the actual update can happen in the
	// controllerutil.CreateOrUpdate call below
	dnsEndpoint := externaldns.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateName(ch),
			Namespace: ch.ResourceNamespace,
		},
	}

	// Update (and not patch) so a concurrent challenge for the same record is
	// detected by the resource version instead of silently overwritten
	var result controllerutil.OperationResult
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		result, err = controllerutil.CreateOrUpdate(ctx, c.client, &dnsEndpoint, func() error {
			// No set identifier: providers that cannot store one never match
			// such an endpoint with the record they created for it, and try to
			// create that record again on every sync
			ep := endpoint.NewEndpointWithTTL(ch.ResolvedFQDN, endpoint.RecordTypeTXT, cfg.ttl(),
				addKey(challengeKeys(&dnsEndpoint, ch), ch.Key)...)
			ep.ProviderSpecific = cfg.ProviderSpecific
			dnsEndpoint.Spec.Endpoints = []*endpoint.Endpoint{ep}
			return nil
		})
		return err
	})

	if err != nil {
		return fmt.Errorf("could not create/update DNSEndpoint object: %w", err)
	}

	//exhaustive:ignore
	switch result {
	case controllerutil.OperationResultCreated:
		logf.Log.Info("created DNSEndpoint object", "request", ch.UID, "namespace", dnsEndpoint.Namespace, "name", dnsEndpoint.Name)
	case controllerutil.OperationResultUpdated:
		logf.Log.Info("updated DNSEndpoint object", "request", ch.UID, "namespace", dnsEndpoint.Namespace, "name", dnsEndpoint.Name)
	case controllerutil.OperationResultNone:
		// Object already existed and was up to date
	}

	return nil
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *externalDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	// Create child context that times out
	ctx, cancel := context.WithTimeout(c.ctx, RequestTimeout)
	defer cancel()

	key := client.ObjectKey{Namespace: ch.ResourceNamespace, Name: generateName(ch)}

	// Remove only this challenge's key, the object goes when no key is left
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var dnsEndpoint externaldns.DNSEndpoint
		if err := c.client.Get(ctx, key, &dnsEndpoint); err != nil {
			// We do not care if the object does not exist
			return client.IgnoreNotFound(err)
		}

		keys := removeKey(challengeKeys(&dnsEndpoint, ch), ch.Key)
		if len(keys) == 0 {
			// The precondition keeps a key that a concurrent Present just added
			err := c.client.Delete(ctx, &dnsEndpoint, client.Preconditions{
				UID:             &dnsEndpoint.UID,
				ResourceVersion: &dnsEndpoint.ResourceVersion,
			})
			return client.IgnoreNotFound(err)
		}

		for _, ep := range dnsEndpoint.Spec.Endpoints {
			if isChallengeEndpoint(ep, ch) {
				ep.Targets = keys
			}
		}
		return c.client.Update(ctx, &dnsEndpoint)
	})
	if err != nil {
		return fmt.Errorf("could not clean up DNSEndpoint object: %w", err)
	}

	// Releases up to 0.0.1 created one object per challenge key, remove the
	// one a challenge that was presented before the upgrade left behind
	legacy := externaldns.DNSEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateLegacyName(ch),
			Namespace: ch.ResourceNamespace,
		},
	}
	if err := c.client.Delete(ctx, &legacy); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("could not delete DNSEndpoint object: %w", err)
	}

	return nil
}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initialising
// connections or warming up caches.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (c *externalDNSProviderSolver) Initialize(config *rest.Config, stopCh <-chan struct{}) error {
	cli, err := client.New(config, client.Options{Scheme: scheme.NewScheme()})
	if err != nil {
		return fmt.Errorf("could not create kubernetes client: %w", err)
	}

	c.client = cli
	c.ctx = contextutil.StopChannelContext(stopCh)

	return nil
}

// generateName returns a short, consistent Kubernetes object name for the
// record of a given challenge. Challenges for the same record (a domain and
// its wildcard) get the same name, so they end up in one DNSEndpoint.
func generateName(ch *v1alpha1.ChallengeRequest) string {
	return namePrefix(ch) + hashString(strings.ToLower(strings.TrimSuffix(ch.ResolvedFQDN, ".")))
}

// generateLegacyName returns the per-key object name used up to release 0.0.1
func generateLegacyName(ch *v1alpha1.ChallengeRequest) string {
	return namePrefix(ch) + hashString(ch.Key)
}

// namePrefix returns the domain name for readability, we remove any wildcards
// as they are invalid characters for a Kubernetes object name. We do not care
// about collisions in the prefix as a suffix with a hash is appended.
func namePrefix(ch *v1alpha1.ChallengeRequest) string {
	return strings.TrimPrefix(
		strings.TrimSuffix(ch.DNSName, "."), "*.") + "-"
}

func hashString(s string) string {
	hash := fnv.New32()
	fmt.Fprint(hash, s)
	return rand.SafeEncodeString(fmt.Sprint(hash.Sum32()))
}

// isChallengeEndpoint reports whether the endpoint is the TXT record of the
// given challenge
func isChallengeEndpoint(ep *endpoint.Endpoint, ch *v1alpha1.ChallengeRequest) bool {
	return ep.RecordType == endpoint.RecordTypeTXT &&
		strings.EqualFold(strings.TrimSuffix(ep.DNSName, "."), strings.TrimSuffix(ch.ResolvedFQDN, "."))
}

// challengeKeys returns the challenge keys the DNSEndpoint object currently
// presents for the record of the given challenge
func challengeKeys(dnsEndpoint *externaldns.DNSEndpoint, ch *v1alpha1.ChallengeRequest) []string {
	var keys []string
	for _, ep := range dnsEndpoint.Spec.Endpoints {
		if isChallengeEndpoint(ep, ch) {
			keys = append(keys, ep.Targets...)
		}
	}
	return keys
}

// addKey returns the keys with the given key added, sorted and without
// duplicates so presenting a challenge twice changes nothing
func addKey(keys []string, key string) []string {
	keys = append(removeKey(keys, key), key)
	slices.Sort(keys)
	return slices.Compact(keys)
}

// removeKey returns the keys without the given key
func removeKey(keys []string, key string) []string {
	return slices.DeleteFunc(slices.Clone(keys), func(k string) bool { return k == key })
}

// DefaultTTL is the TTL of the challenge record in seconds, unless the solver
// config sets one. A challenge record is short-lived, so it should not be
// cached for long.
const DefaultTTL = 60

// solverConfig is the configuration of the webhook solver on the Issuer
type solverConfig struct {
	ProviderSpecific endpoint.ProviderSpecific `json:"providerSpecific,omitempty"`
	TTL              int64                     `json:"ttl,omitempty"`
}

func (c *solverConfig) ttl() endpoint.TTL {
	if c.TTL <= 0 {
		return DefaultTTL
	}
	return endpoint.TTL(c.TTL)
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(cfgJSON *extapi.JSON) (*solverConfig, error) {
	cfg := &solverConfig{}
	if cfgJSON == nil {
		return cfg, nil
	}

	if err := json.Unmarshal(cfgJSON.Raw, cfg); err != nil {
		return nil, fmt.Errorf("error decoding solver config: %v", err)
	}

	return cfg, nil
}
