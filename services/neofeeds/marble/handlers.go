// Package neofeeds provides HTTP handlers for the price feed aggregation service.
package neofeeds

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/internal/httputil"
)

// =============================================================================
// HTTP Handlers
// =============================================================================

func (s *Service) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, s.config)
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
	httputil.WriteJSON(w, http.StatusOK, sources)
}

func (s *Service) handleGetPrice(w http.ResponseWriter, r *http.Request) {
	pair := mux.Vars(r)["pair"]
	if pair == "" {
		httputil.BadRequest(w, "pair required")
		return
	}

	price, err := s.GetPrice(r.Context(), pair)
	if err != nil {
		httputil.InternalError(w, err.Error())
		return
	}

	httputil.WriteJSON(w, http.StatusOK, price)
}

func (s *Service) handleGetPrices(w http.ResponseWriter, r *http.Request) {
	if s.DB() == nil {
		httputil.WriteJSON(w, http.StatusOK, []PriceResponse{})
		return
	}

	// Use configured feeds, not hardcoded DefaultFeeds
	enabledFeeds := s.GetEnabledFeeds()
	var responses []PriceResponse
	for i := range enabledFeeds {
		feed := &enabledFeeds[i]
		if latest, err := s.DB().GetLatestPrice(r.Context(), feed.ID); err == nil {
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
	httputil.WriteJSON(w, http.StatusOK, responses)
}

func (s *Service) handleListFeeds(w http.ResponseWriter, r *http.Request) {
	// Return configured feeds, not sources
	enabledFeeds := s.GetEnabledFeeds()
	feeds := make([]map[string]interface{}, 0, len(enabledFeeds))
	for i := range enabledFeeds {
		feed := &enabledFeeds[i]
		feeds = append(feeds, map[string]interface{}{
			"id":       feed.ID,
			"pair":     feed.Pair,
			"enabled":  feed.Enabled,
			"decimals": feed.Decimals,
		})
	}
	httputil.WriteJSON(w, http.StatusOK, feeds)
}
