package main

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// Упрощённые структуры OpenRTB
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

type BidderResponse struct {
	Cpm float64 `json:"cpm"`
}

func dspHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var bidReq OpenRTBBidRequest
	if err := json.Unmarshal(body, &bidReq); err != nil {
		logToKafka("dsp", "ERROR", "Invalid JSON in OpenRTB request")
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	logToKafka("dsp", "INFO", "Received OpenRTB request: "+bidReq.ID)

	cpm, err := getBidFromBidder(dspConfig.Dsp.BidderURL)
	if err != nil {
		logToKafka("dsp", "ERROR", "Failed to get cpm from Bidder: "+err.Error())
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	adm := `<div>My Ad</div>`
	var bids []Bid
	for _, imp := range bidReq.Imp {
		bids = append(bids, Bid{
			ID:    "dsp-bid-1",
			ImpID: imp.ID,
			Price: cpm,
			AdM:   adm,
		})
	}

	seat := SeatBid{Bid: bids}
	resp := OpenRTBBidResponse{
		ID:    bidReq.ID,
		SeatB: []SeatBid{seat},
		Cur:   "USD",
	}

	respData, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respData)
}

func getBidFromBidder(bidderURL string) (float64, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(bidderURL + "/getBid")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, err
	}

	var bResp BidderResponse
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &bResp); err != nil {
		return 0, err
	}

	return bResp.Cpm, nil
}
