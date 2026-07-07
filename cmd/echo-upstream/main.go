package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

func main() {
	var count atomic.Int32

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200) // health hep 200
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		fmt.Printf("request #%d: %s %s\n", n, r.Method, r.URL.Path)
		if n <= 999 {
			fmt.Println("  -> 500")
			w.WriteHeader(500)
			return
		}
		fmt.Println("  -> 200")
		w.WriteHeader(200)
		fmt.Fprintln(w, `{"ok":true}`)
	})

	fmt.Println("echo-upstream on :8081")
	http.ListenAndServe(":8081", nil)
}
