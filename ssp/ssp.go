package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Ads struct {
		Ssp struct {
			Port         int      `yaml:"port"`
			MaxBids      int      `yaml:"maxBids"`
			MaxTimeoutMs int      `yaml:"maxTimeoutMs"`
			DspEndpoints []string `yaml:"dspEndpoints"`
		} `yaml:"ssp"`
	} `yaml:"ads"`

	Kafka struct {
		BootstrapServers []string `yaml:"bootstrap_servers"`
	} `yaml:"kafka"`
}

type OpenRTBBidRequest struct {
	ID   string `json:"id"`
	Imp  []Imp  `json:"imp"`
	Site Site   `json:"site"`
}

type Imp struct {
	ID     string  `json:"id"`
	Banner *Banner `json:"banner,omitempty"`
}

type Banner struct {
	W int `json:"w"`
	H int `json:"h"`
}

type Site struct {
	Page string `json:"page"`
}

type OpenRTBBidResponse struct {
	ID    string    `json:"id"`
	SeatB []SeatBid `json:"seatbid"`
	Cur   string    `json:"cur"`
}

type SeatBid struct {
	Bid []Bid `json:"bid"`
}

type Bid struct {
	ID    string  `json:"id"`
	ImpID string  `json:"impid"`
	Price float64 `json:"price"`
	AdM   string  `json:"adm"`
}

var sspConfig Config
var producer sarama.SyncProducer

func loadConfig() {
	f, err := os.Open("/app/config.yaml")
	if err != nil {
		log.Fatalf("Failed to open config: %v", err)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&sspConfig); err != nil {
		log.Fatalf("Failed to decode yaml: %v", err)
	}
}

func initKafkaProducer() {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true

	p, err := sarama.NewSyncProducer(sspConfig.Kafka.BootstrapServers, cfg)
	if err != nil {
		log.Fatalf("Failed to init Kafka producer: %v", err)
	}
	producer = p
}

func logToKafka(service, level, message string) {
	now := time.Now().UTC().Format(time.RFC3339)
	msgValue := fmt.Sprintf(`{"timestamp":"%s","service":"%s","level":"%s","message":"%s"}`,
		now, service, level, message)
	_, _, err := producer.SendMessage(&sarama.ProducerMessage{
		Topic: "service-logs",
		Value: sarama.StringEncoder(msgValue),
	})
	if err != nil {
		log.Printf("Kafka send error: %v", err)
	}
}

func main() {
	loadConfig()
	initKafkaProducer()
	logToKafka("ssp", "INFO", "SSP service started")

	http.HandleFunc("/ssp/auction", sspAuctionHandler)

	port := sspConfig.Ads.Ssp.Port
	addr := ":" + strconv.Itoa(port)
	log.Printf("SSP listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func sspAuctionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		logToKafka("ssp", "ERROR", "Failed to read request body")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var bidReq OpenRTBBidRequest
	if err := json.Unmarshal(body, &bidReq); err != nil {
		logToKafka("ssp", "ERROR", "Invalid JSON in request")
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	logToKafka("ssp", "INFO", "Received auction request: "+bidReq.ID)

	dspResponses := getBidsFromAllDSP(bidReq)
	allBids := gatherAllBids(dspResponses)

	// Сортируем по Price (убывание)
	for i := 0; i < len(allBids); i++ {
		for j := i + 1; j < len(allBids); j++ {
			if allBids[j].Price > allBids[i].Price {
				allBids[i], allBids[j] = allBids[j], allBids[i]
			}
		}
	}

	maxBids := sspConfig.Ads.Ssp.MaxBids
	if len(allBids) > maxBids {
		allBids = allBids[:maxBids]
	}

	finalResp := OpenRTBBidResponse{
		ID:    bidReq.ID,
		SeatB: []SeatBid{{Bid: allBids}},
		Cur:   "USD",
	}

	respData, _ := json.Marshal(finalResp)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respData)
	logToKafka("ssp", "INFO", fmt.Sprintf("Auction response with %d bids", len(allBids)))
}

func getBidsFromAllDSP(bidReq OpenRTBBidRequest) []OpenRTBBidResponse {
	dspList := sspConfig.Ads.Ssp.DspEndpoints
	var wg sync.WaitGroup
	wg.Add(len(dspList))

	ch := make(chan *OpenRTBBidResponse, len(dspList))

	for _, dspURL := range dspList {
		go func(url string) {
			defer wg.Done()
			resp := sendOpenRtbRequest(url, bidReq)
			ch <- resp
		}(dspURL)
	}

	wg.Wait()
	close(ch)

	var responses []OpenRTBBidResponse
	for r := range ch {
		if r != nil {
			responses = append(responses, *r)
		}
	}
	return responses
}

func sendOpenRtbRequest(url string, bidReq OpenRTBBidRequest) *OpenRTBBidResponse {
	data, err := json.Marshal(bidReq)
	if err != nil {
		logToKafka("ssp", "ERROR", "Failed to marshal request: "+err.Error())
		return nil
	}

	client := &http.Client{Timeout: time.Duration(sspConfig.Ads.Ssp.MaxTimeoutMs) * time.Millisecond}
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		logToKafka("ssp", "ERROR", "Failed to create DSP POST request: "+err.Error())
		return nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		logToKafka("ssp", "ERROR", "DSP request failed: "+err.Error())
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logToKafka("ssp", "WARN", fmt.Sprintf("DSP returned code %d", resp.StatusCode))
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logToKafka("ssp", "ERROR", "Failed to read DSP response: "+err.Error())
		return nil
	}

	var dspResp OpenRTBBidResponse
	if err := json.Unmarshal(body, &dspResp); err != nil {
		logToKafka("ssp", "ERROR", "Invalid JSON from DSP: "+err.Error())
		return nil
	}

	return &dspResp
}

func gatherAllBids(dspResponses []OpenRTBBidResponse) []Bid {
	var all []Bid
	for _, r := range dspResponses {
		for _, seat := range r.SeatB {
			all = append(all, seat.Bid...)
		}
	}
	return all
}

// Вспомогательные функции
func ioReadAllN(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}

func ioReadAll(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body)
}

func main() {
	loadConfig()
	initKafkaProducer()
	logToKafka("ssp", "INFO", "SSP service started")

	http.HandleFunc("/ssp/auction", sspAuctionHandler)

	port := sspConfig.Ads.Ssp.Port
	addr := ":" + strconv.Itoa(port)
	log.Printf("SSP listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
