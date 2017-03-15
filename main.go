package main

import (
	"log"
	"net/http"
)

func initT(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("[]"))
}

func main() {

	host := ":8080"

	http.HandleFunc("/init", initT)
	log.Println("Started listening on ", host)
	log.Fatal(http.ListenAndServe(host, nil))
}
