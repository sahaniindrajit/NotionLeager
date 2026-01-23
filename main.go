package main

import (
	"log"
	"net/http"
	"os"
)

func main() {

	port := os.Getenv("PORT")

	if port == "8080" {
		port="8080"
	}

	http.HandleFunc("/",func(w http.ResponseWriter,r *http.Request){
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	log.Println("Listening on port", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}