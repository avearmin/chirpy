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
	polkaApiKey    string
}

func main() {
	const root = "."
	const appDir = "./app"
	const port = "8080"

	godotenv.Load()

	apiCfg := &apiConfig{
		fileserverHits: 0,
		jwtSecret:      os.Getenv("JWT_SECRET"),
		polkaApiKey:    os.Getenv("POLKA_API_KEY"),
	}

	router := chi.NewRouter()
	fshandler := apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(appDir))))
	router.Handle("/app/*", fshandler)
	router.Handle("/app", fshandler)

	apiRouter := chi.NewRouter()
	apiRouter.Get("/healthz", readinessEndpointHandler)
	apiRouter.Get("/reset", apiCfg.resetHandler)
	apiRouter.Post("/chirps", apiCfg.postChirpsHandler)
	apiRouter.Get("/chirps", getChirpsHandler)
	apiRouter.Get("/chirps/{id}", getChirpIdHandler)
	apiRouter.Delete("/chirps/{id}", apiCfg.deleteChirpHandler)
	apiRouter.Post("/users", postUsersHandler)
	apiRouter.Put("/users", apiCfg.updateUserCredsHandler)
	apiRouter.Post("/login", apiCfg.postLoginHandler)
	apiRouter.Post("/refresh", apiCfg.postRefreshHandler)
	apiRouter.Post("/revoke", apiCfg.postRevokeHandler)
	apiRouter.Post("/polka/webhooks", apiCfg.postPolkaWebhookHandler)

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

func (cfg *apiConfig) postChirpsHandler(w http.ResponseWriter, r *http.Request) {
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
		respondParseTokenError(w, err)
		return
	}
	if issuer != "chirpy-access" {
		w.WriteHeader(401)
		return
	}
	id, err := parsedToken.Claims.GetSubject()
	if err != nil {
		respondParseTokenError(w, err)
		return
	}

	type parameters struct {
		Body string `json:"body"`
		Id   int    `json:"id"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		respondParamsDecodingError(w, err)
		return
	}

	if len(params.Body) > 140 {
		w.WriteHeader(400)
		return
	}

	db, err := database.NewDB("./database.gob")
	if err != nil {
		respondDatabaseError(w, err)
		return
	}

	numericId, err := strconv.Atoi(id)
	if err != nil {
		respondStrconvError(w, err)
		return
	}
	chirp, err := db.CreateChirp(numericId, cleanChirp(params.Body))
	if err != nil {
		respondDataWriteError(w, err)
		return
	}

	data, err := json.Marshal(chirp)
	if err != nil {
		respondJSONMarshalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(data)
}

func getChirpsHandler(w http.ResponseWriter, r *http.Request) {
	db, err := database.NewDB("./database.gob")
	if err != nil {
		respondDatabaseError(w, err)
		return
	}
	sort := r.URL.Query().Get("sort")
	id := r.URL.Query().Get("author_id")
	var chirps []database.Chirp
	if id != "" {
		numericId, err := strconv.Atoi(id)
		if err != nil {
			respondStrconvError(w, err)
			return
		}
		chirps, err = db.GetChirpsFromId(numericId, sort)
		if err != nil {
			respondDataFetchError(w, err)
			return
		}
	} else {
		chirps, err = db.GetChirps(sort)
		if err != nil {
			respondDataFetchError(w, err)
			return
		}
	}

	data, err := json.Marshal(chirps)
	if err != nil {
		respondJSONMarshalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func getChirpIdHandler(w http.ResponseWriter, r *http.Request) {
	urlParam := chi.URLParam(r, "id")
	id, err := strconv.Atoi(urlParam)
	if err != nil {
		respondParseURLError(w, err)
		return
	}
	db, err := database.NewDB("./database.gob")
	if err != nil {
		respondDatabaseError(w, err)
		return
	}
	chirp, ok, err := db.GetChirp(id)
	if err != nil {
		respondDataFetchError(w, err)
		return
	}
	if !ok {
		w.WriteHeader(404)
		return
	}
	data, err := json.Marshal(chirp)
	if err != nil {
		respondJSONMarshalError(w, err)
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
		respondParamsDecodingError(w, err)
		return
	}
	db, err := database.NewDB("./database.gob")
	if err != nil {
		respondDatabaseError(w, err)
		return
	}
	user, err := db.CreateUser(params.Email, params.Password)
	if err != nil {
		respondDataWriteError(w, err)
		return
	}
	data, err := json.Marshal(user)
	if err != nil {
		respondJSONMarshalError(w, err)
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
		respondParamsDecodingError(w, err)
		return
	}
	db, err := database.NewDB("./database.gob")
	if err != nil {
		respondDatabaseError(w, err)
		return
	}
	if err = db.ComparePasswords(params.Password, params.Email); err != nil { // TODO: Better error handling. ErrUserDoesNotExist should return a 404
		log.Printf(err.Error())
		w.WriteHeader(401)
		return
	}

	type returnVal struct {
		IsChirpyRed  bool   `json:"is_chirpy_red"`
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
		respondDatabaseError(w, err)
		return
	}
	accessToken, err := cfg.createSignedAccessToken(user.Id)
	if err != nil {
		respondAccessTokenError(w, err)
		return
	}
	refreshToken, err := cfg.createSignedRefreshToken(user.Id)
	if err != nil {
		respondRefreshTokenError(w, err)
		return
	}
	resp := returnVal{
		IsChirpyRed:  user.IsChirpyRed,
		Email:        user.Email,
		Id:           user.Id,
		Token:        accessToken,
		RefreshToken: refreshToken,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		respondJSONMarshalError(w, err)
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
		respondParseTokenError(w, err)
		return
	}
	if issuer != "chirpy-access" {
		w.WriteHeader(401)
		return
	}
	id, err := parsedToken.Claims.GetSubject()
	if err != nil {
		respondParseTokenError(w, err)
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
		respondParamsDecodingError(w, err)
		return
	}

	type returnVal struct {
		Email string `json:"email"`
		Id    int    `json:"id"`
	}
	db, err := database.NewDB("./database.gob")
	if err != nil {
		respondDatabaseError(w, err)
		return
	}
	numericId, err := strconv.Atoi(id)
	if err != nil {
		respondStrconvError(w, err)
	}
	db.UpdateUser(numericId, params.Email, params.Password)
	resp := returnVal{
		Email: params.Email,
		Id:    numericId,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		respondJSONMarshalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(data)
}

func (cfg *apiConfig) postRefreshHandler(w http.ResponseWriter, r *http.Request) {
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
		respondParseTokenError(w, err)
		return
	}
	if issuer != "chirpy-refresh" {
		w.WriteHeader(401)
		return
	}
	db, err := database.NewDB("./database.gob")
	if err != nil {
		respondDatabaseError(w, err)
		return
	}
	revoked, err := db.IsTokenRevoked(token)
	if err != nil {
		respondDatabaseError(w, err)
		return
	}
	if revoked {
		w.WriteHeader(401)
		return
	}

	type returnVal struct {
		Token string `json:"token"`
	}
	id, err := parsedToken.Claims.GetSubject()
	if err != nil {
		respondParseTokenError(w, err)
		return
	}
	numericId, err := strconv.Atoi(id)
	if err != nil {
		respondStrconvError(w, err)
		return
	}
	newAccessToken, err := cfg.createSignedAccessToken(numericId)
	if err != nil {
		respondAccessTokenError(w, err)
		return
	}
	resp := returnVal{Token: newAccessToken}
	data, err := json.Marshal(resp)
	if err != nil {
		respondJSONMarshalError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(data)
}

func (cfg *apiConfig) postRevokeHandler(w http.ResponseWriter, r *http.Request) {
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
		respondParseTokenError(w, err)
		return
	}
	if issuer != "chirpy-refresh" {
		w.WriteHeader(401)
		return
	}
	db, err := database.NewDB("./database.gob")
	if err != nil {
		respondDatabaseError(w, err)
		return
	}
	revoked, err := db.IsTokenRevoked(token)
	if err != nil {
		respondDatabaseError(w, err)
		return
	}
	if revoked {
		w.WriteHeader(409) // We're indicating a conflict. The token they want to revoke was already revoked
		return
	}
	if err := db.RevokeRefreshToken(token); err != nil {
		respondUnexpectedError(w, err) // We would have already checked for all possible errors this could be, so something unexpected would have to happend to cause this.
		return
	}
	w.WriteHeader(200)
}

func (cfg *apiConfig) deleteChirpHandler(w http.ResponseWriter, r *http.Request) {
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
		respondParseTokenError(w, err)
		return
	}
	if issuer != "chirpy-access" {
		w.WriteHeader(401)
		return
	}
	urlParam := chi.URLParam(r, "id")
	if err != nil {
		respondParseURLError(w, err)
		return
	}
	chirpIdToDelete, err := strconv.Atoi(urlParam)
	if err != nil {
		respondStrconvError(w, err)
		return
	}
	requesterId, err := parsedToken.Claims.GetSubject()
	if err != nil {
		respondParseTokenError(w, err)
		return
	}
	db, err := database.NewDB("./database.gob")
	if err != nil {
		respondDatabaseError(w, err)
		return
	}
	numericRequesterId, err := strconv.Atoi(requesterId)
	if err != nil {
		respondStrconvError(w, err)
		return
	}
	err = db.DeleteChirp(chirpIdToDelete, numericRequesterId)
	if err == database.ErrChirpDoesNotExist {
		w.WriteHeader(404)
		return
	}
	if err == database.ErrAuthorization {
		w.WriteHeader(403)
		return
	}
	if err != nil {
		respondDatabaseError(w, err)
		return
	}
	w.WriteHeader(200)
}

func (cfg *apiConfig) postPolkaWebhookHandler(w http.ResponseWriter, r *http.Request) {
	apiKey := strings.TrimPrefix(r.Header.Get("Authorization"), "ApiKey ")
	if cfg.polkaApiKey != apiKey {
		w.WriteHeader(401)
		return
	}

	type parameters struct {
		Event string `json:"event"`
		Data  struct {
			UserId int `json:"user_id"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondParamsDecodingError(w, err)
		return
	}
	if params.Event != "user.upgraded" {
		w.WriteHeader(200)
		return
	}
	db, err := database.NewDB("./database.gob")
	if err != nil {
		respondDatabaseError(w, err)
		return
	}
	if err := db.UpgradeUser(params.Data.UserId); err != nil {
		w.WriteHeader(404)
		return
	}
	w.WriteHeader(200)

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
