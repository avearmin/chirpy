package database

import (
	"encoding/gob"
	"errors"
	"io/fs"
	"os"
	"sync"
)

type DB struct {
	path string
	mux  *sync.RWMutex
}

type Chirp struct {
	Body string `json:"body"`
	Id   int    `json:"id"`
}

type DBStructure struct {
	NextId int
	Chirps map[int]Chirp
}

func NewDB(path string) (*DB, error) {
	db := DB{
		path: path,
		mux:  &sync.RWMutex{},
	}
	if err := db.ensureDB(); err != nil {
		return nil, err
	}
	return &db, nil
}

func (db *DB) CreateChirp(body string) (Chirp, error) {
	dbStruct, err := db.loadDB()
	if err != nil {
		return Chirp{}, err
	}
	chirp := Chirp{
		Id:   dbStruct.NextId,
		Body: body,
	}
	dbStruct.Chirps[dbStruct.NextId] = chirp
	dbStruct.NextId++
	db.writeDB(dbStruct)
	return chirp, nil
}

func (db *DB) GetChirp(id int) (Chirp, bool, error) {
	dbStruct, err := db.loadDB()
	if err != nil {
		return Chirp{}, false, err
	}
	found, ok := dbStruct.Chirps[id]
	if !ok {
		return Chirp{}, false, nil
	}
	return found, true, nil
}

func (db *DB) GetChirps() ([]Chirp, error) {
	dbStruct, err := db.loadDB()
	if err != nil {
		return nil, err
	}
	keys := make([]Chirp, len(dbStruct.Chirps))
	i := 0
	for id := range dbStruct.Chirps {
		keys[i] = dbStruct.Chirps[id]
		i++
	}
	return keys, nil
}

func (db *DB) ensureDB() error {
	if exists(db.path) {
		return nil
	}
	_, err := os.Create(db.path)
	if err != nil {
		return err
	}
	dbStruct := DBStructure{
		NextId: 1,
		Chirps: make(map[int]Chirp),
	}
	if err := db.writeDB(dbStruct); err != nil {
		return err
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	return true
}

func (db *DB) loadDB() (DBStructure, error) {
	dbStruct := DBStructure{}
	db.mux.RLocker().Lock()
	defer db.mux.RLocker().Unlock()
	file, err := os.Open(db.path)
	if err != nil {
		return DBStructure{}, err
	}
	defer file.Close()
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&dbStruct); err != nil {
		return DBStructure{}, err
	}
	return dbStruct, nil
}

func (db *DB) writeDB(dbStructure DBStructure) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	file, err := os.OpenFile(db.path, os.O_WRONLY|os.O_TRUNC, 0664)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(dbStructure); err != nil {
		return err
	}
	return nil
}

func (db *DB) GetNextAvailableID() (int, error) {
	dbStruct, err := db.loadDB()
	if err != nil {
		return 0, err
	}
	return dbStruct.NextId, nil
}
