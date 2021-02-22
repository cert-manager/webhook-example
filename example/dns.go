package example

import (
	"fmt"
	"github.com/miekg/dns"
)

func (e *exampleSolver) handleDNSRequest(w dns.ResponseWriter, req *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(req)
	switch req.Opcode {
	case dns.OpcodeQuery:
		for _, q := range msg.Question {
			switch q.Qtype {
			case dns.TypeA:
				rr, err := dns.NewRR(fmt.Sprintf("%s 5 IN A 127.0.0.1", q.Name))
				if err != nil {
					msg.SetRcode(req, dns.RcodeNameError)
				} else {
					msg.Answer = append(msg.Answer, rr)
				}
			case dns.TypeTXT:
				// get record
				e.RLock()
				record, found := e.txtRecords[q.Name]
				e.RUnlock()
				if !found {
					msg.SetRcode(req, dns.RcodeNameError)
				} else {
					rr, err := dns.NewRR(fmt.Sprintf("%s 5 IN TXT %s", q.Name, record))
					if err != nil {
						msg.SetRcode(req, dns.RcodeServerFailure)
						break
					}
					msg.Answer = append(msg.Answer, rr)
				}
			case dns.TypeNS:
				rr, err := dns.NewRR(fmt.Sprintf("%s 5 IN NS ns.example-acme-webook.invalid.", q.Name))
				if err != nil {
					msg.SetRcode(req, dns.RcodeServerFailure)
					break
				} else {
					msg.Answer = append(msg.Answer, rr)
				}
			case dns.TypeSOA:
				rr, err := dns.NewRR(fmt.Sprintf("%s 5 IN SOA %s 20 5 5 5 5", "ns.example-acme-webook.invalid.", "ns.example-acme-webook.invalid."))
				if err != nil {
					msg.SetRcode(req, dns.RcodeServerFailure)
					break
				}
				msg.Answer = append(msg.Answer, rr)
			default:
				msg.SetRcode(req, dns.RcodeServerFailure)
				break
			}
		}
	}
	w.WriteMsg(msg)
}
