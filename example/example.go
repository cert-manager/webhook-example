// package example contains a self-contained example of a webhook that passes the cert-manager
// DNS conformance tests
package example

import (
	"fmt"
	"os"
	"sync"

	"github.com/jetstack/cert-manager/pkg/acme/webhook"
	acme "github.com/jetstack/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/miekg/dns"
	"k8s.io/client-go/rest"
)

type exampleSolver struct {
	name       string
	server     *dns.Server
	txtRecords map[string]string
	sync.RWMutex
}

func (e *exampleSolver) Name() string {
	return e.name
}

func (e *exampleSolver) Present(ch *acme.ChallengeRequest) error {
	e.Lock()
	e.txtRecords[ch.ResolvedFQDN] = ch.Key
	e.Unlock()
	return nil
}

func (e *exampleSolver) CleanUp(ch *acme.ChallengeRequest) error {
	e.Lock()
	delete(e.txtRecords, ch.ResolvedFQDN)
	e.Unlock()
	return nil
}

func (e *exampleSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	go func(done <-chan struct{}) {
		<-done
		if err := e.server.Shutdown(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		}
	}(stopCh)
	go func() {
		if err := e.server.ListenAndServe(); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	}()
	return nil
}

func New(port string) webhook.Solver {
	e := &exampleSolver{
		name:       "example",
		txtRecords: make(map[string]string),
	}
	e.server = &dns.Server{
		Addr:    ":" + port,
		Net:     "udp",
		Handler: dns.HandlerFunc(e.handleDNSRequest),
	}
	return e
}
