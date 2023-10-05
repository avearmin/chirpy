package main

import (
	"log"
	"net/http"
)

func respondError(w http.ResponseWriter, logMessage string, err error) {
	log.Printf("%s: %s", logMessage, err)
	w.WriteHeader(http.StatusInternalServerError)
}

func respondDatabaseError(w http.ResponseWriter, err error) {
	respondError(w, "Error connecting to database", err)
}

func respondParamsDecodingError(w http.ResponseWriter, err error) {
	respondError(w, "Error decoding parameters", err)
}

func respondStrconvError(w http.ResponseWriter, err error) {
	respondError(w, "Error converting stringified ID from token into type int", err)
}

func respondAccessTokenError(w http.ResponseWriter, err error) {
	respondError(w, "Error creating access token", err)
}

func respondRefreshTokenError(w http.ResponseWriter, err error) {
	respondError(w, "Error creating refresh token", err)
}

func respondDataFetchError(w http.ResponseWriter, err error) {
	respondError(w, "Error fetching data from database", err)
}

func respondDataWriteError(w http.ResponseWriter, err error) {
	respondError(w, "Error writing to database", err)
}

func respondJSONMarshalError(w http.ResponseWriter, err error) {
	respondError(w, "Error marshalling JSON", err)
}

func respondParseTokenError(w http.ResponseWriter, err error) {
	respondError(w, "Error parsing token", err)
}

func respondParseURLError(w http.ResponseWriter, err error) {
	respondError(w, "Error parsing URL", err)
}

func respondUnexpectedError(w http.ResponseWriter, err error) {
	respondError(w, "Something went wrong", err)
}
