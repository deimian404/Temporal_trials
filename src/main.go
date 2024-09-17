// main.go
package main

import (
	"fmt"
	"github.com/miekg/dns"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/worker"
	"go.uber.org/zap"
	"log/slog"
	"net/http"
	"os"
	"temporal_setup/src/dns_server"
)

var (
	HostPort  = "127.0.0.1:7233"
	TaskQueue = "dnsTaskQueue"
)

func main() {
	logger := buildLogger()
	temporalClient := buildTemporalClient(logger)
	startWorker(logger, temporalClient)

	handler := &dns_server.DNSHandler{
		TemporalClient: temporalClient,
	}

	server := &dns.Server{
		Addr:      ":1053",
		Net:       "udp",
		Handler:   handler,
		UDPSize:   65535,
		ReusePort: true,
	}

	fmt.Println("Starting DNS server on port 1053")
	err := server.ListenAndServe()
	if err != nil {
		fmt.Printf("Failed to start server: %s\n", err.Error())
	}

	fmt.Println("Starting HTTP server on port 8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}

func buildLogger() log.Logger {
	logger := log.NewStructuredLogger(
		slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		})))

	return logger
}

func buildTemporalClient(logger log.Logger) client.Client {
	c, err := client.Dial(client.Options{
		HostPort:  HostPort,
		Namespace: "dns",
		Logger:    logger,
	})
	if err != nil {
		panic("Failed to create Temporal client")
	}
	return c
}

func startWorker(logger log.Logger, temporalClient client.Client) {
	workerOptions := worker.Options{}

	var w = worker.New(
		temporalClient,
		TaskQueue,
		workerOptions,
	)
	w.RegisterWorkflow(dns_server.DNSWorkflow)
	w.RegisterActivity(dns_server.Resolve)
	w.RegisterActivity(dns_server.PublishToKafka)
	w.RegisterActivity(dns_server.RetrieveFromKafka)

	err := w.Start()
	if err != nil {
		fmt.Println(err)
		panic("Failed to start worker")
	}

	logger.Info("Started Worker", zap.String("worker", TaskQueue))
}
