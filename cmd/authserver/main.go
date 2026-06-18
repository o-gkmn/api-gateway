package main

import (
	"api-gateway/internal/auth"
	"api-gateway/pkg/keys"
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	privateKey, err := keys.LoadRSAPrivateKey("keys/jwt_private.pem")
	if err != nil {
		log.Fatalf("failed to load private key: %v", err)
	}

	jwk := auth.PublicKeyToJWK(&privateKey.PublicKey, "dev")
	jwks := auth.JSONWebKeySet{Keys: []auth.JSONWebKey{jwk}}

	http.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		log.Println("Serving JWKS")
		if err := json.NewEncoder(w).Encode(jwks); err != nil {
			http.Error(w, "Failed to encode JWKS", http.StatusInternalServerError)
		}
	})

	log.Println("Starting auth server on :8081")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatalf("Server failed: %v\n", err)
	}
}
