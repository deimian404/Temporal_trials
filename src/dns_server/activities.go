package dns_server

import (
	"encoding/json"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/miekg/dns"
	"time"
)

// Kafka REST Port | 8082
// Plaintext Ports | 38353

// Resolve queries Google's public DNS server for the given domain and query type.
func Resolve(domain string, qtype uint16) ([]JSONDNSRecord, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), qtype)
	m.RecursionDesired = true

	c := new(dns.Client)
	c.Timeout = 5 * time.Second
	in, _, err := c.Exchange(m, "8.8.8.8:53")
	if err != nil {
		return nil, err
	}

	fmt.Printf("Received DNS answer: %+v\n", in.Answer)

	var jsonAnswers []JSONDNSRecord
	for _, answer := range in.Answer {
		jsonRecord := JSONDNSRecord{
			Type:  dns.TypeToString[answer.Header().Rrtype],
			Value: answer.String(),
		}
		jsonAnswers = append(jsonAnswers, jsonRecord)
	}

	fmt.Printf("JSON DNS Records: %+v\n", jsonAnswers)
	return jsonAnswers, nil
}

var (
	kafkaBrokerAddress = "localhost:9092"
	topicDNSRequests   = "DNS_requests"
	producerInstance   *kafka.Producer
)

func InitKafkaProducer() (*kafka.Producer, error) {
	if producerInstance != nil {
		return producerInstance, nil
	}
	p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": kafkaBrokerAddress})
	if err != nil {
		return nil, err
	}
	producerInstance = p
	return producerInstance, nil
}

func PublishToKafka(request DNSRequest) error {
	producer, err := InitKafkaProducer()
	if err != nil {
		return err
	}

	jsonRequest, err := json.Marshal(request)
	if err != nil {
		return err
	}

	message := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topicDNSRequests, Partition: kafka.PartitionAny},
		Key:            []byte(request.SessionID),
		Value:          jsonRequest,
	}

	// Produce the message to Kafka
	err = producer.Produce(message, nil)
	if err != nil {
		return err
	}

	// Wait for message deliveries, to make sure the message is delivered
	e := <-producer.Events()
	m := e.(*kafka.Message)

	if m.TopicPartition.Error != nil {
		return m.TopicPartition.Error
	}

	fmt.Printf("Pushed DNSRequest to Kafka: %s\n", jsonRequest)
	return nil
}

// RetrieveFromKafka retrieves the DNSResponse from the Kafka topic based on the session ID.
func RetrieveFromKafka(sessionID string, timeout time.Duration) (DNSResponse, error) {
	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": kafkaBrokerAddress,
		"group.id":          "dns_response_group",
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		return DNSResponse{}, err
	}
	defer consumer.Close()

	err = consumer.Subscribe(topicDNSRequests, nil)
	if err != nil {
		return DNSResponse{}, err
	}

	var dnsResponse DNSResponse
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return dnsResponse, fmt.Errorf("timeout waiting for DNS response")
		default:
			msg, err := consumer.ReadMessage(-1)
			if err != nil {
				continue
			}

			fmt.Printf("Popped from Kafka: %s\n", string(msg.Value))

			if string(msg.Key) == sessionID {
				err = json.Unmarshal(msg.Value, &dnsResponse)
				if err != nil {
					continue
				}

				fmt.Printf("Unmarshalled DNS response: %+v\n", dnsResponse)
				return dnsResponse, nil
			}
		}
	}
}

// PushToRedis pushes the DNSRequest to the Redis list.
//func PushToRedis(ctx context.Context, request DNSRequest) error {
//	rdb := redis.NewClient(&redis.Options{
//		Addr:     "localhost:6379",
//		Password: "",
//		DB:       0,
//	})
//
//	jsonRequest, err := json.Marshal(request)
//	if err != nil {
//		return err
//	}
//
//	fmt.Printf("Pushing DNSRequest to Redis: %s\n", jsonRequest)
//	return rdb.LPush(ctx, "dns_requests", jsonRequest).Err()
//}
//
//// RetrieveFromRedis retrieves the DNSResponse from the Redis list based on the session ID.
//func RetrieveFromRedis(ctx context.Context, sessionID string, timeout time.Duration) (DNSResponse, error) {
//	rdb := redis.NewClient(&redis.Options{
//		Addr:     "localhost:6379",
//		Password: "",
//		DB:       0,
//	})
//
//	var dnsResponse DNSResponse
//	timer := time.NewTimer(timeout)
//	defer timer.Stop()
//
//	for {
//		select {
//		case <-timer.C:
//			return dnsResponse, fmt.Errorf("timeout waiting for DNS response")
//		default:
//			result, err := rdb.BRPop(ctx, 5*time.Second, "dns_requests").Result()
//			if err != nil {
//				continue
//			}
//
//			fmt.Printf("Popped from Redis: %s\n", result[1])
//
//			err = json.Unmarshal([]byte(result[1]), &dnsResponse)
//			if err != nil {
//				continue
//			}
//
//			fmt.Printf("Unmarshalled DNS response: %+v\n", dnsResponse)
//
//			if dnsResponse.SessionID == sessionID {
//				return dnsResponse, nil
//			}
//		}
//	}
//}
