package testing

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"

	"github.com/miekg/dns"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

// Registry implements an sigs.k8s.io/external-dns/registry.Registry by hosting
// a local DNS server. This allows us to test directly against ourselves over
// localhost.
type Registry struct {
	mu      sync.RWMutex
	records map[endpoint.EndpointKey]*endpoint.Endpoint
	domains *endpoint.DomainFilter
}

func NewTestRegistry(t *testing.T, addr string, domains ...string) *Registry {
	domainFilter := endpoint.NewDomainFilter(domains)
	registry := &Registry{
		domains: domainFilter,
		records: make(map[endpoint.EndpointKey]*endpoint.Endpoint),
	}

	log.Printf("[TEST REGISTRY] Starting DNS server on %s for domains: %v", addr, domains)

	server := &dns.Server{
		Addr:    addr,
		Net:     "udp",
		Handler: dns.HandlerFunc(registry.handleDNSRequest),
	}

	go func() {
		_ = server.ListenAndServe()
	}()

	t.Cleanup(func() {
		log.Printf("[TEST REGISTRY] Shutting down DNS server")
		_ = server.Shutdown()
	})

	return registry
}

func (r *Registry) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	records := make([]*endpoint.Endpoint, 0, len(r.records))
	for _, record := range r.records {
		records = append(records, record.DeepCopy())
	}

	log.Printf("[TEST REGISTRY] Records() returning %d records", len(records))
	for _, record := range records {
		log.Printf("[TEST REGISTRY]   - %s %s %v", record.DNSName, record.RecordType, record.Targets)
	}

	return records, nil
}

func (r *Registry) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Printf("[TEST REGISTRY] ApplyChanges() - Create: %d, Update: %d, Delete: %d",
		len(changes.Create), len(changes.UpdateNew), len(changes.Delete))

	for _, change := range changes.Create {
		log.Printf("[TEST REGISTRY]   CREATE: %s %s %v", change.DNSName, change.RecordType, change.Targets)
		r.records[change.Key()] = change.DeepCopy()
	}

	for _, change := range changes.UpdateNew {
		log.Printf("[TEST REGISTRY]   UPDATE: %s %s %v", change.DNSName, change.RecordType, change.Targets)
		r.records[change.Key()] = change.DeepCopy()
	}

	for _, change := range changes.Delete {
		log.Printf("[TEST REGISTRY]   DELETE: %s %s %v", change.DNSName, change.RecordType, change.Targets)
		delete(r.records, change.Key())
	}

	return nil
}

func (r *Registry) AdjustEndpoints(endpoints []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	return endpoints, nil
}

func (r *Registry) GetDomainFilter() endpoint.DomainFilterInterface {
	return r.domains
}

func (r *Registry) OwnerID() string {
	return ""
}

func (r *Registry) handleDNSRequest(w dns.ResponseWriter, req *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(req)
	if req.Opcode == dns.OpcodeQuery {
		for _, q := range msg.Question {
			if err := r.addDNSAnswer(q, msg, req); err != nil {
				msg.SetRcode(req, dns.RcodeServerFailure)
				break
			}
		}
	}
	_ = w.WriteMsg(msg)
}

func (r *Registry) addDNSAnswer(q dns.Question, msg *dns.Msg, req *dns.Msg) error {
	log.Printf("[TEST REGISTRY] DNS Query: %s %s", q.Name, dns.TypeToString[q.Qtype])

	switch q.Qtype {
	// Always return loopback for any A query
	case dns.TypeA:
		log.Printf("[TEST REGISTRY]   A record query for %s, returning 127.0.0.1", q.Name)
		rr, err := dns.NewRR(fmt.Sprintf("%s 5 IN A 127.0.0.1", q.Name))
		if err != nil {
			return err
		}
		msg.Answer = append(msg.Answer, rr)
		return nil

	// TXT records are the only important record for ACME dns-01 challenges
	case dns.TypeTXT:
		records, err := r.Records(context.TODO())
		if err != nil {
			return err
		}

		found := false
		queryName := strings.TrimSuffix(q.Name, ".")
		log.Printf("[TEST REGISTRY]   TXT record query for %s", queryName)

		for _, record := range records {
			if record.DNSName == queryName && record.RecordType == endpoint.RecordTypeTXT {
				log.Printf("[TEST REGISTRY]   Found matching record: %s -> %v", record.DNSName, record.Targets)
				for _, target := range record.Targets {
					rr, err := dns.NewRR(fmt.Sprintf("%s 5 IN TXT %s", q.Name, target))
					if err != nil {
						return err
					}

					msg.Answer = append(msg.Answer, rr)
					found = true
				}
			}
		}

		if !found {
			log.Printf("[TEST REGISTRY]   No TXT record found for %s, returning NXDOMAIN", queryName)
			msg.SetRcode(req, dns.RcodeNameError)
			return nil
		}

		return nil

	// NS and SOA are for authoritative lookups, return obviously invalid data
	case dns.TypeNS:
		rr, err := dns.NewRR(fmt.Sprintf("%s 5 IN NS ns.external-dns-acme-webook.invalid.", q.Name))
		if err != nil {
			return err
		}
		msg.Answer = append(msg.Answer, rr)
		return nil
	case dns.TypeSOA:
		rr, err := dns.NewRR(fmt.Sprintf("%s 5 IN SOA %s 20 5 5 5 5", "ns.external-dns-acme-webook.invalid.", "ns.external-dns-acme-webook.invalid."))
		if err != nil {
			return err
		}
		msg.Answer = append(msg.Answer, rr)
		return nil
	default:
		return fmt.Errorf("unimplemented record type %v", q.Qtype)
	}
}
