package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("=== %s %s ===\n", r.Method, r.URL.Path)
		for k, v := range r.Header {
			fmt.Printf("  %s: %s\n", k, v)
		}
		fmt.Println()
		w.Header().Set("Server", "echo-upstream")
		w.WriteHeader(200)
		fmt.Fprintln(w, `{"echo":"ok"}`)
	})
	fmt.Println("Echo upstream on :8081")
	http.ListenAndServe(":8081", nil)
}
