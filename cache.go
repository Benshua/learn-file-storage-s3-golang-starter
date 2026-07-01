package main

import (
	"log"
	"net/http"
) 

func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		log.Printf("setting header to: %v", w.Header().Get("Cache-Control"))
		next.ServeHTTP(w, r)
	})
}
