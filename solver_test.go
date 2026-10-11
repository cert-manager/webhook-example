package main

import (
	"context"
	"testing"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	externaldns "sigs.k8s.io/external-dns/apis/v1alpha1"
	"sigs.k8s.io/external-dns/endpoint"

	"github.com/lion7/cert-manager-webhook-external-dns/internal/scheme"
)

func newFakeSolver() *externalDNSProviderSolver {
	return &externalDNSProviderSolver{
		client: fake.NewClientBuilder().WithScheme(scheme.NewScheme()).Build(),
		ctx:    context.Background(),
	}
}

func challenge(dnsName, key string) *v1alpha1.ChallengeRequest {
	return &v1alpha1.ChallengeRequest{
		ResourceNamespace: "default",
		DNSName:           dnsName,
		ResolvedFQDN:      "_acme-challenge.example.com.",
		Key:               key,
	}
}

func listDNSEndpoints(t *testing.T, solver *externalDNSProviderSolver) []externaldns.DNSEndpoint {
	t.Helper()
	var list externaldns.DNSEndpointList
	if err := solver.client.List(context.Background(), &list, client.InNamespace("default")); err != nil {
		t.Fatalf("could not list DNSEndpoint objects: %v", err)
	}
	return list.Items
}

func assertKeys(t *testing.T, solver *externalDNSProviderSolver, keys ...string) {
	t.Helper()
	items := listDNSEndpoints(t, solver)
	if len(keys) == 0 {
		if len(items) != 0 {
			t.Fatalf("expected no DNSEndpoint objects, got %d", len(items))
		}
		return
	}
	if len(items) != 1 || len(items[0].Spec.Endpoints) != 1 {
		t.Fatalf("expected one DNSEndpoint object with one endpoint, got %+v", items)
	}
	ep := items[0].Spec.Endpoints[0]
	if ep.DNSName != "_acme-challenge.example.com" || ep.RecordType != endpoint.RecordTypeTXT {
		t.Errorf("unexpected record %s %s", ep.DNSName, ep.RecordType)
	}
	if ep.SetIdentifier != "" {
		t.Errorf("expected no set identifier, got %q", ep.SetIdentifier)
	}
	if ep.RecordTTL != DefaultTTL {
		t.Errorf("expected TTL %d, got %d", DefaultTTL, ep.RecordTTL)
	}
	if !ep.Targets.Same(keys) {
		t.Errorf("expected keys %v, got %v", keys, ep.Targets)
	}
}

// A domain and its wildcard are validated on the same record, each with its
// own key.
func TestChallengesForTheSameRecordShareOneEndpoint(t *testing.T) {
	solver := newFakeSolver()
	apex, wildcard := challenge("example.com", "key-b"), challenge("*.example.com", "key-a")

	for _, ch := range []*v1alpha1.ChallengeRequest{apex, wildcard, apex} {
		if err := solver.Present(ch); err != nil {
			t.Fatalf("Present: %v", err)
		}
	}
	assertKeys(t, solver, "key-a", "key-b")

	if err := solver.CleanUp(apex); err != nil {
		t.Fatalf("CleanUp: %v", err)
	}
	assertKeys(t, solver, "key-a")

	for _, ch := range []*v1alpha1.ChallengeRequest{wildcard, wildcard} {
		if err := solver.CleanUp(ch); err != nil {
			t.Fatalf("CleanUp: %v", err)
		}
	}
	assertKeys(t, solver)
}

func TestCleanUpRemovesLegacyObject(t *testing.T) {
	solver := newFakeSolver()
	ch := challenge("example.com", "key-a")

	legacy := externaldns.DNSEndpoint{}
	legacy.Namespace, legacy.Name = ch.ResourceNamespace, generateLegacyName(ch)
	if err := solver.client.Create(context.Background(), &legacy); err != nil {
		t.Fatalf("could not create legacy object: %v", err)
	}

	if err := solver.CleanUp(ch); err != nil {
		t.Fatalf("CleanUp: %v", err)
	}
	assertKeys(t, solver)
}
