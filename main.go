package main

import (
	"chirpy/database"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
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
	jwtSecret      string
	polkaApiSecret string
}

type errorBody struct {
	Error string `json:"error"`
}

type noPasswordUser struct {
	Id    int    `json:"id"`
	Email string `json:"email"`
}

// metrics - counting landing page server hits
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits++
		next.ServeHTTP(w, r)
	})
}

// GET /admin/metrics
// returns an html page embedded with the number of times the `/` page was served
func (cfg *apiConfig) metricsHandlerFunc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Typ", "text/html")
	w.WriteHeader(http.StatusOK)
	htmlTemplate, err := os.ReadFile("./admin/metrics/template.html")
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

// GET /api/chirps
// return the JSON of all the Chirps as a list of Chirps
// takes an optional query parameter `author_id` a user id, if present only return chirps by that author
// e.g. GET http://localhost:8080/api/chirps?author_id=1
// another optional query parameter `sort`, can be either `asc` or `desc`, sorts chirps by id in that order
// default id sorting is by `asc` order
func (apiCfg apiConfig) readChirpsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Request: GET /api/chirps")

	orderScheme := "asc" // default order is ascending

	// see if "sort" param present
	tmp := r.URL.Query().Get("sort")
	if tmp == "desc" {
		orderScheme = "desc"
	}

	// see if author_id is present
	authorId := r.URL.Query().Get("author_id")
	if authorId != "" {
		// return only chirps by author
		authorIdInt, err := strconv.Atoi(authorId)
		if err != nil {
			respondWithError(w, http.StatusNotFound, errors.New("no user/author with that id"))
			log.Println("no user/author with that id")
			return
		}
		respondWithJSON(w, 200, apiCfg.db.GetChirpsByAuthor(authorIdInt, orderScheme))
		return
	}

	// return all chirps if optional author_id param not provided
	allChirps := apiCfg.db.GetChirps(orderScheme)
	respondWithJSON(w, 200, allChirps)
}

// GET /api/chirps/{id}
// return just a single chirp
func (apiCfg apiConfig) readOneChirpHandler(w http.ResponseWriter, r *http.Request) {
	// get chirp id
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		respondWithError(w, 404, err)
		return
	}
	// find the chirp from id if possible
	chirp, err := apiCfg.db.GetChirp(id)
	if err != nil {
		respondWithError(w, 404, err)
		return
	}
	// respond with found chirp matching the given id
	respondWithJSON(w, 200, chirp)
}

// POST /api/chirps
// create new Chirps
func (apiCfg apiConfig) createChirpHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Request: POST /api/chirps")

	// first authenticate user
	_, token, err := apiCfg.getJWTAndValidate(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, errors.New("invalid token"))
		log.Println("invalid token")
		return
	}

	// get the user id
	userId, err := token.Claims.GetSubject()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, errors.New("no id in JWT subject"))
		log.Println("no id in JWT subject")
		return
	}

	// decode the chirp from JSON into go struct
	decoder := json.NewDecoder(r.Body)
	params := database.Chirp{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, errors.New("could not decode your chirp JSON"))
		return
	}

	// attach author_id to the chirp
	authorId, err := strconv.Atoi(userId)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, errors.New("invalid userid in JWT"))
		log.Println("invalid userid in JWT")
		return
	}
	params.Author_id = authorId

	// create the chirp
	newChirp, err := apiCfg.db.CreateChirp(params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err)
		log.Println(err)
		return
	}

	// respond with acknowledgement that chirp was created
	respondWithJSON(w, 201, newChirp)
}

// DELETE /api/chirps/{id}
// delete a chirp by its id, authenticated endpoint
func (apiCfg apiConfig) deleteChirpHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("DELETE /api/chirps/{id}")
	// first authenticate user
	_, token, err := apiCfg.getJWTAndValidate(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, errors.New("invalid token"))
		log.Println("invalid token")
		return
	}

	// get the user id
	userIdString, err := token.Claims.GetSubject()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, errors.New("no id in JWT subject"))
		log.Println("no id in JWT subject")
		return
	}
	userId, err := strconv.Atoi(userIdString)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err)
		log.Println(err)
		return
	}

	// get chirp id from url
	chirpId, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		respondWithError(w, 404, err)
		return
	}

	// make sure chirp's author_id is the same as the JWT's id
	if userId != chirpId {
		respondWithError(w, http.StatusForbidden, errors.New("you are not the author of that chirp"))
		return
	}

	// delete chirp
	err = apiCfg.db.DeleteChirp(chirpId)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err)
		return
	}

	respondWithJSON(w, http.StatusOK, nil)
}

// POST /api/healthz
// healthz -- readiness endpoint
func readinessHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Request: POST /api/healthz")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

// check passwords, return true if strong else false
func isPasswordStrong(password string) bool {
	// Check length
	if len(password) < 8 {
		return false
	}

	// Check for uppercase letters
	hasUpper := false
	for _, c := range password {
		if unicode.IsUpper(c) {
			hasUpper = true
			break
		}
	}
	if !hasUpper {
		return false
	}

	// Check for lowercase letters
	hasLower := false
	for _, c := range password {
		if unicode.IsLower(c) {
			hasLower = true
			break
		}
	}
	if !hasLower {
		return false
	}

	// Check for digits
	hasDigit := false
	for _, c := range password {
		if unicode.IsDigit(c) {
			hasDigit = true
			break
		}
	}
	if !hasDigit {
		return false
	}

	// Check for special characters
	hasSpecial := false
	for _, c := range password {
		if unicode.IsPunct(c) || unicode.IsSymbol(c) {
			hasSpecial = true
			break
		}
	}
	if !hasSpecial {
		return false
	}

	// All tests passed
	return true
}

// remove the password entry from a user struct, return noPasswordUser struct
func removePasswordFromUser(user database.User) noPasswordUser {
	return noPasswordUser{
		Id:    user.Id,
		Email: user.Email,
	}
}

// POST /api/users
// create a new user
// returns noPassUser, fields: (id, email )
func (apiCfg apiConfig) createNewUserHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Request: POST /api/users")
	// decode the user from JSON into go struct
	decoder := json.NewDecoder(r.Body)
	params := database.User{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, errors.New("decoding json went wrong"))
		return
	}

	// check if email is already being used
	users := apiCfg.db.GetUsers()
	for _, user := range users {
		if params.Email == user.Email {
			respondWithError(w, http.StatusNotAcceptable, errors.New("email is already in use"))
			return
		}
	}

	// check password strength
	if !isPasswordStrong(params.Password) {
		respondWithError(w, http.StatusNotAcceptable, errors.New("password is not strong"))
		return
	}

	// create the new user
	newUser := apiCfg.db.CreateNewUser(params)

	// remove the hashed password before sending back
	removedPassUser := removePasswordFromUser(newUser)

	// respond with acknowledgement that user was created
	respondWithJSON(w, 201, removedPassUser)
}

// POST /api/login
// login a user
// returns email, pass, access_token, refresh_token
func (apiCfg apiConfig) authenticateUserHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Request: POST /api/login")
	type parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	// decode the user from JSON into go struct
	decoder := json.NewDecoder(r.Body)
	// params := database.User{}
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, errors.New("error decoding your json"))
		return
	}

	enteredEmail := params.Email
	enteredPassword := params.Password

	// retrieve user by email
	users := apiCfg.db.GetUsers()
	foundUserEntry := false
	userEntryIdx := -1
	for i, user := range users {
		if user.Email == enteredEmail {
			foundUserEntry = true
			userEntryIdx = i
			break
		}
	}

	if !foundUserEntry {
		respondWithError(w, http.StatusInternalServerError, errors.New("no user with that email found in db"))
		return
	}

	foundUser := users[userEntryIdx]

	// compare the password
	err = bcrypt.CompareHashAndPassword([]byte(foundUser.Password), []byte(enteredPassword))
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, errors.New("passwords don't match"))
		return
	}

	// user entered the right password

	// create the JWT with expiration time either given from the user or using a default value
	// create access and refresh tokens

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(1) * time.Hour)),
		Subject:   fmt.Sprintf("%d", foundUser.Id),
	})

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-refresh",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(24*60) * time.Hour)),
		Subject:   fmt.Sprintf("%d", foundUser.Id),
	})

	completeAccessToken, err := accessToken.SignedString([]byte(apiCfg.jwtSecret))
	if err != nil {
		log.Fatal(err)
	}

	completeRefreshToken, err := refreshToken.SignedString([]byte(apiCfg.jwtSecret))
	if err != nil {
		log.Fatal(err)
	}

	type retVal struct {
		Id            int    `json:"id"`
		Email         string `json:"email"`
		Is_chirpy_red bool   `json:"is_chirpy_red"`
		Token         string `json:"token"`         // access token
		Refresh_token string `json:"refresh_token"` // refresh token
	}

	respondWithJSON(w, 200, retVal{
		Id:            foundUser.Id,
		Email:         foundUser.Email,
		Is_chirpy_red: foundUser.Is_chirpy_red,
		Token:         completeAccessToken,
		Refresh_token: completeRefreshToken,
	})
}

// get JWT/APIKEY from the "Authorization" header
// expects format - Authorization: Bearer <token> / Authorization: ApiKey <key>
// where "Authorization" is the header name
func getAuthTokenFromHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	splitAuthHeader := strings.Split(authHeader, " ")
	if len(splitAuthHeader) != 2 {
		return "", errors.New("no token provided")
	}
	tokenString := splitAuthHeader[1]
	return tokenString, nil
}

// validate a JWT based off of the string
// returns the token if valid, else returns nil
// can use the token to get the user id
func (apiCfg apiConfig) validateToken(tokenString string) (*jwt.Token, error) {
	// validate the JWT
	claims := &jwt.RegisteredClaims{}
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		secret := []byte(apiCfg.jwtSecret)
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}

		return secret, nil
	}
	token, err := jwt.ParseWithClaims(tokenString, claims, keyFunc)
	if err != nil {
		log.Println("error parsing token or invalid token: ", err)
		return nil, err
	}

	if !token.Valid {
		return nil, nil
	}
	return token, nil
}

// to avoid more copy pasting, combine getting and validating
// JWT token, return both the tokenString, token
// returns tokenString, token, err
// if invalid or something else, err is not nil
// if err == nil, then ok
func (apiCfg apiConfig) getJWTAndValidate(r *http.Request) (string, *jwt.Token, error) {
	tokenString, err := getAuthTokenFromHeader(r)
	if err != nil {
		return "", nil, err
	}

	token, err := apiCfg.validateToken(tokenString)
	if err != nil {
		return "", nil, err
	}

	return tokenString, token, nil
}

// PUT /api/users
// update a user's email and password
// authenticated endpoint
func (apiCfg apiConfig) updateUserHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Request: PUT /api/users")
	// retrieve the validated JWT token
	_, token, err := apiCfg.getJWTAndValidate(r)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err)
		log.Println(err)
		return
	}

	// check the issuer to see if whether access vs refresh token
	issuer, err := token.Claims.GetIssuer()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, errors.New("could not get issuer from token"))
		log.Println("could not get issuer from token")
		return
	}

	// reject if not an access token
	if issuer != "chirpy-access" {
		respondWithError(w, http.StatusUnauthorized, errors.New("not access token"))
		log.Println("expected access token, got refresh token")
		return
	}

	// get the user id from the token
	userId := token.Claims.(*jwt.RegisteredClaims).Subject
	userIdInt, _ := strconv.Atoi(userId)

	// get the user from the db
	foundUser, err := apiCfg.db.GetUser(userIdInt)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, errors.New("could not find user"))
		return
	}

	// decode the new user data from JSON into go struct
	decoder := json.NewDecoder(r.Body)
	params := database.User{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, errors.New("error decoding given body"))
		return
	}

	// update the user
	foundUser.Email = params.Email
	foundUser.Password = params.Password
	updatedUser := apiCfg.db.UpdateUser(foundUser)

	// remove the hashed password before sending back
	removedPassUser := removePasswordFromUser(updatedUser)

	// respond with acknowledgement that user was updated
	respondWithJSON(w, 200, removedPassUser)
}

// POST /api/refresh
// requires a refresh token and if valid generates and returns an access token
func (apiCfg apiConfig) refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Request: POST /api/refresh")
	// retrieve the validated JWT token
	tokenString, token, err := apiCfg.getJWTAndValidate(r)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err)
		log.Println(err)
		return
	}

	// check issuer
	issuer, err := token.Claims.GetIssuer()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, errors.New("could not get issuer from token"))
		log.Println("could not get issuer from token")
		return
	}

	if issuer != "chirpy-refresh" {
		respondWithError(w, http.StatusUnauthorized, errors.New("not refresh token"))
		log.Println("expected refresh token, got access token")
		return
	}

	// check that there are no revocations for this token in db
	validityStatus := apiCfg.db.CheckRefreshTokenIsValid(tokenString)
	if !validityStatus {
		respondWithError(w, http.StatusUnauthorized, errors.New("refresh token has been revoked"))
		log.Println("refresh token has been revoked")
		return
	}

	// refresh token ok, create a new access token
	userId := token.Claims.(*jwt.RegisteredClaims).Subject

	newAccessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(1) * time.Hour)),
		Subject:   userId,
	})

	completeAccessToken, _ := newAccessToken.SignedString([]byte(apiCfg.jwtSecret))

	type retVal struct {
		Token string `json:"token"` // access token
	}

	// respond with the new access token
	respondWithJSON(w, 200, retVal{Token: completeAccessToken})
}

// POST /api/revoke
// requires a refresh token and if valid revokes it
func (apiCfg apiConfig) revokeRefreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Request: POST /api/revoke")
	// retrieve the validated JWT token
	tokenString, token, err := apiCfg.getJWTAndValidate(r)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err)
		log.Println(err)
		return
	}

	// check it is a refresh token
	issuer, err := token.Claims.GetIssuer()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, errors.New("could not get issuer from token"))
		log.Println("could not get issuer from token")
		return
	}

	if issuer != "chirpy-refresh" {
		respondWithError(w, http.StatusUnauthorized, errors.New("not refresh token"))
		log.Println("expected refresh token, got access token; issuer: ", issuer)
		return
	}

	// revoke it
	apiCfg.db.RevokeRefreshToken(tokenString)
	// ensure that it is revoked
	if status := apiCfg.db.CheckRefreshTokenIsValid(tokenString); status {
		log.Fatal("token should've been revoked but didn't -- need to fix func")
	}

	// respond with OK
	respondWithJSON(w, http.StatusOK, nil)
}

// POST /api/polka/webhooks
// upgrade a user to Chirpy Red if they are upgrading
// requires polka's api key for authentication
func (apiCfg apiConfig) polkaWebhooksHandler(w http.ResponseWriter, r *http.Request) {
	// validate polka api key before doing anything
	apiKeyString, err := getAuthTokenFromHeader(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err)
		return
	}
	if apiKeyString != apiCfg.polkaApiSecret {
		respondWithError(w, http.StatusUnauthorized, nil)
		return
	}

	type parameter struct {
		Event string `json:"event"`
		Data  struct {
			User_id int `json:"user_id"`
		} `json:"data"`
	}

	// decode the user from JSON into go struct
	decoder := json.NewDecoder(r.Body)
	params := parameter{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, errors.New("decoding json went wrong"))
		return
	}

	// any event other than user.upgraded, just respond with OK
	if params.Event != "user.upgraded" {
		respondWithJSON(w, http.StatusOK, nil)
		return
	}

	// event is user is upgraded
	userId := params.Data.User_id
	err = apiCfg.db.UpgradeUserToChirpyRed(userId)
	if err != nil {
		respondWithError(w, http.StatusNotFound, err)
		return
	}

	respondWithJSON(w, http.StatusOK, nil)
}

// main
func main() {
	filepathRoot := "."
	databaseFile := "database.json"
	godotenv.Load() // load .env
	jwtSecret := os.Getenv("JWT_SECRET")
	polkaAPIKeySecret := os.Getenv("POLKA_KEY")

	// if in debug mode, delete the database.json file if it exists
	dbg := flag.Bool("debug", false, "Enable debug mode")
	flag.Parse()
	log.Println("Debug mode (delete previous db):", *dbg)
	if *dbg {
		e := os.Remove(databaseFile)
		if e != nil {
			log.Println(e)
			return
		}
	}

	// create the DB
	db, err := database.NewDB(databaseFile) // creates and loads the db
	if err != nil {
		log.Fatal(err)
	}
	apiCfg := &apiConfig{
		fileserverHits: 0,
		db:             db,
		jwtSecret:      jwtSecret,
		polkaApiSecret: polkaAPIKeySecret,
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

	apiRouter.Post("/chirps", apiCfg.createChirpHandler)        // create new chirps
	apiRouter.Get("/chirps", apiCfg.readChirpsHandler)          // get all chirps
	apiRouter.Delete("/chirps/{id}", apiCfg.deleteChirpHandler) // delete a chirp
	apiRouter.Get("/chirps/{id}", apiCfg.readOneChirpHandler)   // read a single chirp

	apiRouter.Post("/users", apiCfg.createNewUserHandler)       // create a new User
	apiRouter.Put("/users", apiCfg.updateUserHandler)           // update a User
	apiRouter.Post("/refresh", apiCfg.refreshTokenHandler)      // create new access token using a refresh token
	apiRouter.Post("/revoke", apiCfg.revokeRefreshTokenHandler) // revoke a refresh token

	apiRouter.Post("/login", apiCfg.authenticateUserHandler) // authenticate User

	apiRouter.Post("/polka/webhooks", apiCfg.polkaWebhooksHandler) // polka is "payment provider", pinging this whenever a user has upgraded to Chirpy Red

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
