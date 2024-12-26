package main

import (
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
	Dsp struct {
		Port      int    `yaml:"port"`
		BidderURL string `yaml:"bidderURL"`
	} `yaml:"dsp"`

	Kafka struct {
		BootstrapServers []string `yaml:"bootstrap_servers"`
	} `yaml:"kafka"`
}

var dspConfig Config
var producer sarama.SyncProducer

func loadConfig() {
	f, err := os.Open("/app/config.yaml")
	if err != nil {
		log.Fatalf("Error opening config.yaml: %v", err)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&dspConfig); err != nil {
		log.Fatalf("Error decoding yaml: %v", err)
	}
}

func initKafkaProducer() {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	p, err := sarama.NewSyncProducer(dspConfig.Kafka.BootstrapServers, cfg)
	if err != nil {
		log.Fatalf("Failed to start Kafka producer: %v", err)
	}
	producer = p
}

func logToKafka(service, level, message string) {
	now := time.Now().UTC().Format(time.RFC3339)
	msgValue := fmt.Sprintf(
		`{"timestamp":"%s","service":"%s","level":"%s","message":"%s"}`,
		now, service, level, message)
	producer.SendMessage(&sarama.ProducerMessage{
		Topic: "service-logs",
		Value: sarama.StringEncoder(msgValue),
	})
}

func main() {
	loadConfig()
	initKafkaProducer()
	logToKafka("dsp", "INFO", "DSP service started")

	http.HandleFunc("/openrtb2/auction", dspHandler)

	addr := ":" + strconv.Itoa(dspConfig.Dsp.Port)
	log.Printf("DSP is listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
