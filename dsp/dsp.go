package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/Shopify/sarama"
)

type Config struct {
	Dsp struct {
		Port      int    `yaml:"port"`
		BidderURL string `yaml:"bidderURL"`
	} `yaml:"dsp"`

	Kafka struct {
		Brokers []string `yaml:"bootstrap_servers"`
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
	p, err := sarama.NewSyncProducer(dspConfig.Kafka.Brokers, cfg)
	if err != nil {
		log.Fatalf("Failed to start Kafka producer: %v", err)
	}
	producer = p
}

func logToKafka(service, level, message string) {
	// Аналогично Bidder
}

// dspHandler в dsp_handler.go (см. ниже)

func main() {
	loadConfig()
	initKafkaProducer()
	logToKafka("dsp", "INFO", "DSP service started")

	http.HandleFunc("/openrtb2/auction", dspHandler)

	addr := ":" + strconv.Itoa(dspConfig.Dsp.Port)
	log.Printf("DSP is listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
