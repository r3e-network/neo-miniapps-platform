// Package neofeeds provides HTTP handlers for the price feed aggregation service.
package neofeeds

import (
	"encoding/json"
	"net/http"

	"github.com/R3E-Network/service_layer/internal/httputil"
)

// =============================================================================
// HTTP Handlers
// =============================================================================

func (s *Service) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.config)
}

func (s *Service) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources := make([]map[string]interface{}, 0, len(s.sources))
	for id, src := range s.sources {
		sources = append(sources, map[string]interface{}{
			"id":     id,
			"name":   src.Name,
			"weight": src.Weight,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sources)
}

func (s *Service) handleGetPrice(w http.ResponseWriter, r *http.Request) {
	// Extract pair from URL (e.g., /price/BTCUSDT)
	pair := r.URL.Path[len("/price/"):]
	if pair == "" {
		httputil.BadRequest(w, "pair required")
		return
	}

	price, err := s.GetPrice(r.Context(), pair)
	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(price)
}

func (s *Service) handleGetPrices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.DB() == nil {
		json.NewEncoder(w).Encode([]PriceResponse{})
		return
	}

	var responses []PriceResponse
	for _, feedID := range DefaultFeeds {
		if latest, err := s.DB().GetLatestPrice(r.Context(), feedID); err == nil {
			responses = append(responses, PriceResponse{
				FeedID:    latest.FeedID,
				Pair:      latest.Pair,
				Price:     latest.Price,
				Decimals:  latest.Decimals,
				Timestamp: latest.Timestamp,
				Sources:   latest.Sources,
				Signature: latest.Signature,
			})
		}
	}
	json.NewEncoder(w).Encode(responses)
}

func (s *Service) handleListFeeds(w http.ResponseWriter, r *http.Request) {
	feeds := make([]map[string]string, 0, len(s.sources))
	for id, src := range s.sources {
		feeds = append(feeds, map[string]string{
			"id":   id,
			"name": src.Name,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(feeds)
}
