package main

import (
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

// metrics - counting landing page server hits
type apiConfig struct {
	fileserverHits int
}

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

type errorBody struct {
	Error string `json:"error"`
}

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

// validate new chirps
func validateChirpHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	type returnVals struct {
		Cleaned_body string `json:"cleaned_body"`
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

	// reply with censored chirp
	badWordReplacement := "****"
	listOfBadWords := []string{"kerfuffle", "sharbert", "fornax"}
	cleanedChirp := censorChirp(listOfBadWords, params.Body, badWordReplacement)
	respondWithJSON(w, http.StatusOK, returnVals{Cleaned_body: cleanedChirp})
}

// main
func main() {
	filepathRoot := "."
	apiCfg := &apiConfig{fileserverHits: 0}

	// chi router -- use it to stop extra HTTP methods from working, restrict to GETs
	r := chi.NewRouter()
	r.Mount("/", apiCfg.middlewareMetricsInc(http.FileServer(http.Dir(filepathRoot))))

	// ------------ api ---------------
	// api router
	apiRouter := chi.NewRouter()
	r.Mount("/api", apiRouter)

	// readiness endpoint
	apiRouter.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})

	apiRouter.Post("/validate_chirp", validateChirpHandler)

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
