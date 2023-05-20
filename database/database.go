package database

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
)

type DB struct {
	path   string
	mux    *sync.RWMutex
	Chirps map[int]Chirp `json:"chirps"`
}

type Chirp struct {
	Id   int    `json:"id"`
	Body string `json:"body"`
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
	db := DB{
		path:   path,
		mux:    &sync.RWMutex{},
		Chirps: make(map[int]Chirp), // need to allocate mem here to decode JSON into later
	}

	// load the JSON file contents into mem
	db.loadDB()

	return &db, nil
}

// CreateChirp creates a new chirp and saves it to disk
func (db *DB) CreateChirp(body string) int {
	// only one Writer at a time to create new Chirps
	db.mux.Lock()
	defer db.mux.Unlock()

	// get new id
	maxKey := 0
	for key := range db.Chirps {
		if key > maxKey {
			maxKey = key
		}
	}
	newKey := maxKey + 1

	// create the chirp and add it to the db
	db.Chirps[newKey] = Chirp{
		Id:   newKey,
		Body: body,
	}

	// write the new DB from mem to disk
	db.writeDB()

	// return the new chirp id
	return newKey
}

// GetChirp returns a SINGLE chirp from the database, if you know the id
func (db *DB) GetChirp(id int) (Chirp, error) {
	// lock for Readers
	db.mux.RLock()
	defer db.mux.RUnlock()

	// get chirp if exists
	chirp, ok := db.Chirps[id]
	if !ok {
		return Chirp{}, fmt.Errorf("chirp with ID %d not found", id)
	}

	return chirp, nil
}

// GetChirps returns all chirps in the database
// order by id in ascending order
func (db *DB) GetChirps() ([]Chirp, error) {
	// lock for Readers
	db.mux.RLock()
	defer db.mux.RUnlock()

	// get the list of chirps
	chirps := []Chirp{}
	for key := range db.Chirps {
		chirps = append(chirps, db.Chirps[key])
	}

	// Sort slice of Chirp objects by ID
	sort.Slice(chirps, func(i, j int) bool {
		return chirps[i].Id < chirps[j].Id
	})

	return chirps, nil
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

	// Decode JSON data into DB.Chirps map
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&db.Chirps); err != nil {
		return err
	}

	return nil
}

// writeDB writes the database file to disk
// used by CreateChirps
func (db *DB) writeDB() error {
	// get the JSON file
	file, err := os.OpenFile(db.path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// write the Chirps map to the JSON file
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(db.Chirps); err != nil {
		return err
	}

	return nil
}
