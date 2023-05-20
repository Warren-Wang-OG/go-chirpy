package main

import (
	"chirpy/database"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
)

// allows cross origin requests
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

type apiConfig struct {
	fileserverHits int
	db             *database.DB
}

type errorBody struct {
	Error string `json:"error"`
}

// metrics - counting landing page server hits
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits++
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metricsHandlerFunc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Typ", "text/html")
	w.WriteHeader(http.StatusOK)
	htmlTemplate, err := ioutil.ReadFile("./admin/metrics/template.html")
	if err != nil {
		log.Fatalf("received err: %v", err)
	}
	htmlData := fmt.Sprintf(string(htmlTemplate), cfg.fileserverHits)
	w.Write([]byte(htmlData))
}

// -------------

// wrapper for respondWithJSON for sending errors as the interface used to be converted to json
func respondWithError(w http.ResponseWriter, code int, err error) {
	respondWithJSON(w, code, errorBody{Error: err.Error()})
}

// handles http requests and return json
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	response, err := json.Marshal(payload)
	if err != nil {
		log.Fatal(err)
	}
	w.Write(response)
}

// return the JSON of all the chirps as a list
func (apiCfg apiConfig) readChirpsHandler(w http.ResponseWriter, r *http.Request) {
	allChirps, err := apiCfg.db.GetChirps()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err)
		log.Fatal(err)
	}
	respondWithJSON(w, 200, allChirps)
}

// censors bad words from a string by replacing them with some censor
// listOfBadWords is a list of strings of bad words to censor
// s is the string to sensor
// badWordReplacement is the censor string (replacement val)
func censorChirp(listOfBadWords []string, s, badWordReplacement string) string {
	chirpWords := strings.Split(s, " ")
	for i, word := range chirpWords {
		for _, badWord := range listOfBadWords {
			if strings.ToLower(word) == badWord {
				chirpWords[i] = badWordReplacement
			}
		}
	}
	return strings.Join(chirpWords, " ")
}

// create new Chirps
func (apiCfg apiConfig) createChirpHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	// decode the chirp from JSON into go struct
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		// an error will be thrown if the JSON is invalid or has the wrong types
		// any missing fields will simply have their values in the struct set to their zero value
		respondWithError(w, http.StatusInternalServerError, errors.New("Something went wrong"))
		return
	}

	// check if chirp is too long
	if len(params.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, errors.New("Chirp is too long"))
		return
	}

	// censor chirp
	badWordReplacement := "****"
	listOfBadWords := []string{"kerfuffle", "sharbert", "fornax"}
	cleanedChirp := censorChirp(listOfBadWords, params.Body, badWordReplacement)

	// create the chirp
	newChirpId := apiCfg.db.CreateChirp(cleanedChirp)

	// respond with acknowledgement that chirp created
	newChirp, err := apiCfg.db.GetChirp(newChirpId)
	if err != nil {
		log.Fatal(err)
	}
	respondWithJSON(w, 201, newChirp)
}

// healthz
func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

// main
func main() {
	filepathRoot := "."
	databaseFile := "database.json"

	// create the DB
	db, err := database.NewDB(databaseFile) // creates and loads the db
	if err != nil {
		log.Fatal(err)
	}
	apiCfg := &apiConfig{
		fileserverHits: 0,
		db:             db,
	}

	// chi router -- use it to stop extra HTTP methods from working, restrict to GETs
	r := chi.NewRouter()
	r.Mount("/", apiCfg.middlewareMetricsInc(http.FileServer(http.Dir(filepathRoot))))

	// ------------ api ---------------
	// api router
	apiRouter := chi.NewRouter()
	r.Mount("/api", apiRouter)

	// readiness endpoint
	apiRouter.Get("/healthz", readinessHandler)

	// create new chirps
	apiRouter.Post("/chirps", apiCfg.createChirpHandler)

	// read chirps
	apiRouter.Get("/chirps", apiCfg.readChirpsHandler)

	// ------------ api ---------------

	// admin
	adminRouter := chi.NewRouter()
	r.Mount("/admin", adminRouter)

	// metrics: number of serve requests
	adminRouter.Get("/metrics", apiCfg.metricsHandlerFunc)

	// wrap the main chi router with a handler function that allows CORS
	corsMux := middlewareCors(r)

	// create server and listen
	srv := http.Server{
		Addr:    ":8080",
		Handler: corsMux,
	}
	srv.ListenAndServe()
}
