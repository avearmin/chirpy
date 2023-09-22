package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/avearmin/chirpy/internal/database"
	"github.com/go-chi/chi/v5"
)

type apiConfig struct {
	fileserverHits int
}

func main() {
	const root = "."
	const appDir = "./app"
	const port = "8080"

	apiCfg := &apiConfig{
		fileserverHits: 0,
	}

	router := chi.NewRouter()
	fshandler := apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(appDir))))
	router.Handle("/app/*", fshandler)
	router.Handle("/app", fshandler)

	apiRouter := chi.NewRouter()
	apiRouter.Get("/healthz", readinessEndpointHandler)
	apiRouter.Get("/reset", apiCfg.resetHandler)
	apiRouter.Post("/chirps", postChirpsHandler)
	apiRouter.Get("/chirps", getChirpsHandler)

	router.Mount("/api", apiRouter)

	adminRouter := chi.NewRouter()
	adminRouter.Get("/metrics", apiCfg.fileServerHitsHandler)
	router.Mount("/admin", adminRouter)

	corsMux := middlewareCors(router)
	server := &http.Server{
		Addr:    ":" + port,
		Handler: corsMux,
	}

	log.Printf("Serving files from %s on port: %s\n", appDir, port)
	server.ListenAndServe()
}

func readinessEndpointHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

func (cfg *apiConfig) fileServerHitsHandler(w http.ResponseWriter, r *http.Request) {
	htmlContent := fmt.Sprintf(`
        <html>
          <body>
            <h1>Welcome, Chirpy Admin</h1>
            <p>Chirpy has been visited %d times!</p>
          </body>
        </html>`, cfg.fileserverHits)

	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(htmlContent))
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits = 0
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits++
		next.ServeHTTP(w, r)
	})
}

func postChirpsHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
		Id   int    `json:"id"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	if len(params.Body) > 140 {
		w.WriteHeader(400)
		return
	}

	db, err := database.NewDB("./database.gob")
	if err != nil {
		log.Printf("Error connecting to database: %s", err)
		w.WriteHeader(500)
		return
	}

	chirp, err := db.CreateChirp(cleanChirp(params.Body))
	if err != nil {
		log.Printf("Error writing to database: %s", err)
		w.WriteHeader(500)
		return
	}

	data, err := json.Marshal(chirp)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(data)
}

func getChirpsHandler(w http.ResponseWriter, r *http.Request) {
	db, err := database.NewDB("./database.gob")
	if err != nil {
		log.Printf("Error connecting to database: %s", err)
		w.WriteHeader(500)
		return
	}
	chirps, err := db.GetChirps()
	if err != nil {
		log.Printf("Error accessing database: %s", err)
		w.WriteHeader(500)
		return
	}
	data, err := json.Marshal(chirps)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func middlewareCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func cleanChirp(chirp string) string {
	chirpWords := strings.Split(chirp, " ")
	var cleanChirpWords []string
	for _, word := range chirpWords {
		cleanChirpWords = append(cleanChirpWords, cleanWord(word))
	}
	return strings.Join(cleanChirpWords, " ")
}

func cleanWord(word string) string {
	dirtyWords := []string{"kerfuffle", "sharbert", "fornax"}
	for _, dirtyWord := range dirtyWords {
		if strings.ToLower(word) == dirtyWord {
			return "****"
		}
	}
	return word
}
