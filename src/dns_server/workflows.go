package dns_server

import (
	"fmt"
	"github.com/miekg/dns"
	"go.temporal.io/sdk/workflow"
	"time"
)

// DNSWorkflow processes the DNS query and manages the lifecycle.
func DNSWorkflow(ctx workflow.Context, domain string, qtype uint16) (DNSResponse, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute,
		StartToCloseTimeout:    time.Minute,
	})

	var (
		answers     []JSONDNSRecord
		dnsRequest  DNSRequest
		dnsResponse DNSResponse
	)

	fmt.Printf("Starting DNS resolution for domain: %s, type: %v\n", domain, dns.TypeToString[qtype])

	// Resolve DNS
	err := workflow.ExecuteActivity(ctx, Resolve, domain, qtype).Get(ctx, &answers)
	if err != nil {
		return dnsResponse, err
	}

	fmt.Printf("Resolved DNS answers: %+v\n", answers)

	// Collect all ip addresses from the DNS response
	var ips []string
	for _, answer := range answers {
		if aRecord, err := dns.NewRR(answer.Value); err == nil {
			// Collect A and AAAA records
			switch record := aRecord.(type) {
			case *dns.A:
				ips = append(ips, record.A.String())
			case *dns.AAAA:
				ips = append(ips, record.AAAA.String())
			}
		}
	}

	fmt.Printf("Collected IPs: %v\n", ips)

	// define sessionID
	sessionID := fmt.Sprintf("%d", workflow.Now(ctx).UnixNano())
	dnsRequest = DNSRequest{
		SessionID:  sessionID,
		DomainName: domain,
		IPs:        ips,
	}

	fmt.Printf("Generated DNSRequest: %+v\n", dnsRequest)

	// Push DNS Request to Redis
	err = workflow.ExecuteActivity(ctx, PublishToKafka, dnsRequest).Get(ctx, nil)
	if err != nil {
		return dnsResponse, err
	}

	fmt.Println("Published DNSRequest to Kafka")

	// retrieve DNS response from redis
	err = workflow.ExecuteActivity(ctx, RetrieveFromKafka, dnsRequest.SessionID, 5*time.Second).Get(ctx, &dnsResponse)
	if err != nil {
		return dnsResponse, err
	}

	fmt.Printf("Retrieved DNSResponse: %+v\n", dnsResponse)

	return dnsResponse, nil

}
