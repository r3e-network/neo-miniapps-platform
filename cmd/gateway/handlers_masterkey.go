package main

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/internal/marble"
)

func masterKeyHandler(m *marble.Marble) http.HandlerFunc {
	upstream := proxyHandler("neoaccounts", m)
	return func(w http.ResponseWriter, r *http.Request) {
		r = mux.SetURLVars(r, map[string]string{"path": "master-key"})
		upstream(w, r)
	}
}
