package main

import (
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	fileServer := http.FileServer(http.Dir("."))

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.Handle("/", fileServer)
	mux.Handle("/assets/logo.png", fileServer)

	server.ListenAndServe()
}
