package main

import (
	"log"
	"net/http"
)

func respondWithError(w http.ResponseWriter, logMessage string, err error, status int) {
	log.Printf("%s: %s", logMessage, err)
	w.WriteHeader(status)
}

func respondWithInternalError(w http.ResponseWriter, logMessage string, err error) {
	respondWithError(w, logMessage, err, http.StatusInternalServerError)
}

func respondWithDatabaseError(w http.ResponseWriter, err error) {
	respondWithInternalError(w, "Error connecting to database", err)
}
