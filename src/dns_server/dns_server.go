package dns_server

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/miekg/dns"
	"go.temporal.io/sdk/client"
)

// dnsHandler struct to handle DNS requests

func (h *DNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	fmt.Println("Received DNS query")
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

	for _, question := range r.Question {
		fmt.Printf("Received query: %s\n", question.Name)

		// Start DNSWorkflow
		workflowOptions := client.StartWorkflowOptions{
			TaskQueue:                "dnsTaskQueue",
			WorkflowExecutionTimeout: time.Minute,
		}
		we, err := h.TemporalClient.ExecuteWorkflow(context.Background(), workflowOptions, DNSWorkflow, question.Name, question.Qtype)
		if err != nil {
			fmt.Printf("Unable to start workflow: %v\n", err)
			continue
		}

		// fetch workflow result
		var dnsResponse DNSResponse
		err = we.Get(context.Background(), &dnsResponse)
		if err != nil {
			fmt.Printf("Error getting response from workflow: %v\n", err)
			continue
		}

		fmt.Printf("DNS Response: %+v\n", dnsResponse)

		// Convert IPs to dns.RR and append to response message
		for _, ip := range dnsResponse.IPs {
			aRecord := &dns.A{
				Hdr: dns.RR_Header{
					Name:   dnsResponse.DomainName,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    0,
				},
				A: net.ParseIP(ip),
			}
			msg.Answer = append(msg.Answer, aRecord)
		}

		// create a TXT record with the sessionID
		txtRecord := &dns.TXT{
			Hdr: dns.RR_Header{
				Name:   dnsResponse.DomainName,
				Rrtype: dns.TypeTXT,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			Txt: []string{dnsResponse.SessionID},
		}

		msg.Extra = append(msg.Extra, txtRecord)

	}

	fmt.Println("Sending DNS response")
	err := w.WriteMsg(msg)
	if err != nil {
		return
	}
}
