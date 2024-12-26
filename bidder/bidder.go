package main

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/IBM/sarama"
)

type Config struct {
	Bidder struct {
		Port       int     `yaml:"port"`
		DefaultCpm float64 `yaml:"defaultCpm"`
	} `yaml:"bidder"`

	Kafka struct {
		BootstrapServers []string `yaml:"bootstrap_servers"`
	} `yaml:"kafka"`
}

type BidResponse struct {
	Cpm float64 `json:"cpm"`
}

var appConfig Config
var producer sarama.SyncProducer

func loadConfig() {
	file, err := os.Open("/app/config.yaml")
	if err != nil {
		log.Fatalf("Could not open config.yaml: %v", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&appConfig); err != nil {
		log.Fatalf("Error decoding YAML: %v", err)
	}
}

func initKafkaProducer() {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	p, err := sarama.NewSyncProducer(appConfig.Kafka.BootstrapServers, cfg)
	if err != nil {
		log.Fatalf("Failed to start Kafka producer: %v", err)
	}
	producer = p
}

func logToKafka(service, level, message string) {
	event := fmt.Sprintf(
		`{"timestamp":"%s","service":"%s","level":"%s","message":"%s"}`,
		time.Now().UTC().Format(time.RFC3339), service, level, message)
	msg := &sarama.ProducerMessage{
		Topic: "service-logs",
		Value: sarama.StringEncoder(event),
	}
	_, _, err := producer.SendMessage(msg)
	if err != nil {
		log.Printf("Kafka send error: %v", err)
	}
}

func getBidHandler(w http.ResponseWriter, r *http.Request) {
	logToKafka("bidder", "INFO", "Received /getBid request")
	resp := BidResponse{Cpm: appConfig.Bidder.DefaultCpm}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	loadConfig()
	initKafkaProducer()
	logToKafka("bidder", "INFO", "Bidder service started")

	http.HandleFunc("/getBid", getBidHandler)

	addr := ":" + strconv.Itoa(appConfig.Bidder.Port)
	log.Printf("Bidder is listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
