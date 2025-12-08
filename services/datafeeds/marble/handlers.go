// Package datafeeds provides HTTP handlers for the price feed aggregation service.
package datafeedsmarble

import (
	"encoding/json"
	"net/http"
)

// =============================================================================
// HTTP Handlers
// =============================================================================

func (s *Service) handleInfo(w http.ResponseWriter, r *http.Request) {
	enabledFeeds := s.GetEnabledFeeds()
	feedIDs := make([]string, len(enabledFeeds))
	for i, f := range enabledFeeds {
		feedIDs[i] = f.ID
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "active",
		"version":         Version,
		"sources":         len(s.sources),
		"feeds":           feedIDs,
		"update_interval": s.updateInterval.String(),
		"chain_push":      s.enableChainPush,
		"service_fee":     ServiceFeePerUpdate,
	})
}

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
		http.Error(w, "pair required", http.StatusBadRequest)
		return
	}

	price, err := s.GetPrice(r.Context(), pair)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
