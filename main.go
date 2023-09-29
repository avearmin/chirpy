package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/avearmin/chirpy/internal/database"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

type apiConfig struct {
	fileserverHits int
	jwtSecret      string
}

func main() {
	const root = "."
	const appDir = "./app"
	const port = "8080"

	godotenv.Load()

	apiCfg := &apiConfig{
		fileserverHits: 0,
		jwtSecret:      os.Getenv("JWT_SECRET"),
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
	apiRouter.Get("/chirps/{id}", getChirpIdHandler)
	apiRouter.Post("/users", postUsersHandler)
	apiRouter.Post("/login", apiCfg.postLoginHandler)
	apiRouter.Put("/users", apiCfg.updateUserCredsHandler)

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

func getChirpIdHandler(w http.ResponseWriter, r *http.Request) {
	urlParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(urlParam)
	if err != nil {
		log.Printf("Error getting ID from url: %s", err)
		w.WriteHeader(500)
		return
	}
	db, err := database.NewDB("./database.gob")
	if err != nil {
		log.Printf("Error accessing database: %s", err)
		w.WriteHeader(500)
		return
	}
	chirp, ok, err := db.GetChirp(id)
	if err != nil {
		log.Printf("Error accessing database: %s", err)
		w.WriteHeader(500)
		return
	}
	if !ok {
		w.WriteHeader(404)
		return
	}
	data, err := json.Marshal(chirp)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(data)
}

func postUsersHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}
	db, err := database.NewDB("./database.gob")
	if err != nil {
		log.Printf("Error connecting to database: %s", err)
		w.WriteHeader(500)
		return
	}
	user, err := db.CreateUser(params.Email, params.Password)
	if err != nil {
		log.Printf("Error writing to database: %s", err)
		w.WriteHeader(500)
		return
	}
	data, err := json.Marshal(user)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(data)
}

func (cfg *apiConfig) postLoginHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}
	db, err := database.NewDB("./database.gob")
	if err != nil {
		log.Printf("Error connecting to database: %s", err)
		w.WriteHeader(500)
		return
	}
	if err = db.ComparePasswords(params.Password, params.Email); err != nil { // TODO: Better error handling. ErrUserDoesNotExist should return a 404
		log.Printf(err.Error())
		w.WriteHeader(401)
		return
	}

	type returnVal struct {
		Email        string `json:"email"`
		Id           int    `json:"id"`
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	user, err := db.GetUser(params.Email)
	if err == database.ErrUserDoesNotExist {
		w.WriteHeader(404)
		return
	}
	if err != nil {
		log.Printf("Error connecting to database: %s", err)
		w.WriteHeader(500)
		return
	}
	accessToken, err := cfg.createSignedAccessToken(user.Id)
	if err != nil {
		log.Printf("Error creating access token: %s", err)
		w.WriteHeader(500)
		return
	}
	refreshToken, err := cfg.createSignedRefreshToken(user.Id)
	if err != nil {
		log.Printf("Error creating refresh token: %s", err)
		w.WriteHeader(500)
		return
	}
	resp := returnVal{
		Email:        user.Email,
		Id:           user.Id,
		Token:        accessToken,
		RefreshToken: refreshToken,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(data)
}

func (cfg *apiConfig) updateUserCredsHandler(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	claims := jwt.MapClaims{}
	parsedToken, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.jwtSecret), nil
	})
	if err != nil {
		w.WriteHeader(401)
		return
	}
	issuer, err := parsedToken.Claims.GetIssuer()
	if err != nil {
		w.WriteHeader(500)
		return
	}
	if issuer != "chirpy-access" {
		w.WriteHeader(401)
		return
	}
	id, err := parsedToken.Claims.GetSubject()
	if err != nil {
		log.Printf("Eror getting id from token: %s", err)
		w.WriteHeader(500)
		return
	}

	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	type returnVal struct {
		Email string `json:"email"`
		Id    int    `json:"id"`
	}
	db, err := database.NewDB("./database.gob")
	if err != nil {
		log.Printf("Error connecting to database: %s", err)
		w.WriteHeader(500)
		return
	}
	numericId, err := strconv.Atoi(id)
	if err != nil {
		log.Printf("Error converting stringified ID from token into type int: %s", err)
		w.WriteHeader(500)
	}
	db.UpdateUser(numericId, params.Email, params.Password)
	resp := returnVal{
		Email: params.Email,
		Id:    numericId,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
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

func (cfg *apiConfig) createSignedAccessToken(id int) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		Subject:   strconv.Itoa(id),
	})
	signedToken, err := token.SignedString([]byte(cfg.jwtSecret))
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

func (cfg *apiConfig) createSignedRefreshToken(id int) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-refresh",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add((60 * 24) * time.Hour)),
		Subject:   strconv.Itoa(id),
	})
	signedToken, err := token.SignedString([]byte(cfg.jwtSecret))
	if err != nil {
		return "", err
	}
	return signedToken, nil
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
