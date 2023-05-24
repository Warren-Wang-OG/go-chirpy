package database

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

type DB struct {
	path     string
	mux      *sync.RWMutex
	dbstruct *DBStructure
}

type DBStructure struct {
	Users                map[int]User    `json:"users"`
	Chirps               map[int]Chirp   `json:"chirps"`
	RevokedRefreshTokens map[string]bool `json:"revoked_refresh_tokens"`
}

type Chirp struct {
	Id   int    `json:"id"`
	Body string `json:"body"`
}

type User struct {
	Id       int    `json:"id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// CheckRefreshToken checks if a refresh token is revoked
// returns true if not revoked, false if revoked
func (db *DB) CheckRefreshTokenIsValid(token string) bool {
	db.mux.RLock()
	defer db.mux.RUnlock()

	_, ok := db.dbstruct.RevokedRefreshTokens[token]
	return !ok
}

// RevokeRefreshToken adds a refresh token to the revoked list
func (db *DB) RevokeRefreshToken(token string) {
	db.mux.Lock()
	defer db.mux.Unlock()

	db.dbstruct.RevokedRefreshTokens[token] = true
	db.writeDB()
}

// NewDB creates a new database connection
// and creates the database file if it doesn't exist
func NewDB(path string) (*DB, error) {
	// create JSON file if doesn't exist
	_, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	// create the DB struct
	dbStruct := DBStructure{
		Users:  make(map[int]User), // need to allocate mem here to decode JSON into later
		Chirps: make(map[int]Chirp),
	}

	db := DB{
		path:     path,
		mux:      &sync.RWMutex{},
		dbstruct: &dbStruct,
	}

	// load the JSON file contents into mem
	db.loadDB()

	return &db, nil
}

// CreateNewUser creates a new user and saves it to disk
func (db *DB) CreateNewUser(user User) User {
	// only one Writer at a time can create new Users
	db.mux.Lock()
	defer db.mux.Unlock()

	// get new id
	maxId := 0
	for id := range db.dbstruct.Users {
		if id > maxId {
			maxId = id
		}
	}
	newId := maxId + 1

	// add in the id
	user.Id = newId

	// store the hashed password
	hashedPassBytes, err := bcrypt.GenerateFromPassword([]byte(user.Password), 13)
	if err != nil {
		log.Fatal(err)
	}
	user.Password = string(hashedPassBytes)

	// save newUser to mem and disk
	db.dbstruct.Users[newId] = user
	db.writeDB()

	return user
}

// CreateChirp creates a new chirp and saves it to disk
func (db *DB) CreateChirp(body string) Chirp {
	// only one Writer at a time can create new Chirps
	db.mux.Lock()
	defer db.mux.Unlock()

	// get new id
	maxId := 0
	for id := range db.dbstruct.Chirps {
		if id > maxId {
			maxId = id
		}
	}
	newId := maxId + 1

	// create the chirp and add it to the db
	newChirp := Chirp{
		Id:   newId,
		Body: body,
	}

	// save newChirp to mem and disk
	db.dbstruct.Chirps[newId] = newChirp
	db.writeDB()

	return newChirp
}

// UpdateUser updates a user in the database
func (db *DB) UpdateUser(user User) User {
	// only one Writer at a time can update Users
	db.mux.Lock()
	defer db.mux.Unlock()

	// store the hashed password
	hashedPassBytes, err := bcrypt.GenerateFromPassword([]byte(user.Password), 13)
	if err != nil {
		log.Fatal(err)
	}
	user.Password = string(hashedPassBytes)

	// save newUser to mem and disk
	db.dbstruct.Users[user.Id] = user
	db.writeDB()

	return user
}

// GetUser returns a SINGLE user from the database, if you know the id
func (db *DB) GetUser(id int) (User, error) {
	// lock for Readers
	db.mux.RLock()
	defer db.mux.RUnlock()

	// get user if exists
	user, ok := db.dbstruct.Users[id]
	if !ok {
		return User{}, fmt.Errorf("user with ID %d not found", id)
	}

	return user, nil
}

// GetUsers returns a list of Users in database
// no order
func (db *DB) GetUsers() []User {
	// lock for Readers
	db.mux.RLock()
	defer db.mux.RUnlock()

	users := []User{}
	for id := range db.dbstruct.Users {
		users = append(users, db.dbstruct.Users[id])
	}

	return users
}

// GetChirp returns a SINGLE chirp from the database, if you know the id
func (db *DB) GetChirp(id int) (Chirp, error) {
	// lock for Readers
	db.mux.RLock()
	defer db.mux.RUnlock()

	// get chirp if exists
	chirp, ok := db.dbstruct.Chirps[id]
	if !ok {
		return Chirp{}, fmt.Errorf("chirp with ID %d not found", id)
	}

	return chirp, nil
}

// GetChirps returns all chirps in the database
// order by id in ascending order
func (db *DB) GetChirps() []Chirp {
	// lock for Readers
	db.mux.RLock()
	defer db.mux.RUnlock()

	// get the list of chirps
	chirps := []Chirp{}
	for key := range db.dbstruct.Chirps {
		chirps = append(chirps, db.dbstruct.Chirps[key])
	}

	// Sort slice of Chirp objects by ID
	sort.Slice(chirps, func(i, j int) bool {
		return chirps[i].Id < chirps[j].Id
	})

	return chirps
}

// loadDB reads the database file into memory
// used by NewDB after ensuring the file exists / is created
func (db *DB) loadDB() error {
	// lock for Readers
	db.mux.RLock()
	defer db.mux.RUnlock()

	// get the JSON from file, decode into the db.Chirps struct
	// Open file for reading
	file, err := os.Open(db.path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Decode JSON data into db.dbstruct
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&db.dbstruct); err != nil {
		return err
	}

	return nil
}

// writeDB writes the database file to disk
// used by CreateChirps and CreateNewUser
func (db *DB) writeDB() error {
	// get the JSON file
	file, err := os.OpenFile(db.path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// write the dbstruct to the JSON file
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(db.dbstruct); err != nil {
		return err
	}

	return nil
}
