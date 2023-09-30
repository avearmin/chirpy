package database

import (
	"encoding/gob"
	"errors"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Errors raised by package database
var (
	ErrUserAlreadyExists   = errors.New("This user already exists.")
	ErrUserDoesNotExist    = errors.New("User not found.")
	ErrTokenAlreadyRevoked = errors.New("Token is already revoked.")
	ErrChirpDoesNotExist   = errors.New("Chirp not found.")
	ErrAuthorization       = errors.New("This action is not authorized.")
)

type DB struct {
	path string
	mux  *sync.RWMutex
}

type Chirp struct {
	Body     string `json:"body"`
	Id       int    `json:"id"`
	AuthorId int    `json:"author_id"`
}

type User struct {
	Email       string `json:"email"`
	Password    []byte `json:"-"` // Should be encoded into Gob but not JSON
	Id          int    `json:"id"`
	IsChirpyRed bool   `json:"is_chirpy_red"`
}

type DBStructure struct {
	NextChirpId          int
	NextUserId           int
	Chirps               map[int]Chirp
	Users                map[int]User
	RevokedRefreshTokens map[string]time.Time
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

func (db *DB) CreateChirp(createdBy int, body string) (Chirp, error) {
	dbStruct, err := db.loadDB()
	if err != nil {
		return Chirp{}, err
	}
	chirp := Chirp{
		Id:       dbStruct.NextChirpId,
		AuthorId: createdBy,
		Body:     body,
	}
	dbStruct.Chirps[dbStruct.NextChirpId] = chirp
	dbStruct.NextChirpId++
	db.writeDB(dbStruct)
	return chirp, nil
}

func (db *DB) DeleteChirp(chirpIdToDelete, idOfRequestingUser int) error {
	dbStruct, err := db.loadDB()
	if err != nil {
		return err
	}
	chirp, found := dbStruct.Chirps[chirpIdToDelete]
	if !found {
		return ErrChirpDoesNotExist
	}
	if chirp.AuthorId != idOfRequestingUser {
		return ErrAuthorization
	}
	delete(dbStruct.Chirps, chirpIdToDelete)
	if err := db.writeDB(dbStruct); err != nil {
		return err
	}
	return nil
}

func (db *DB) CreateUser(email, password string) (User, error) {
	dbStruct, err := db.loadDB()
	if err != nil {
		return User{}, err
	}
	normalizedEmail := normalizeEmail(email)
	if _, exists := dbStruct.Users[dbStruct.NextUserId]; exists {
		return User{}, ErrUserAlreadyExists
	}
	hashPass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	user := User{
		Id:          dbStruct.NextUserId,
		Email:       normalizedEmail,
		Password:    hashPass,
		IsChirpyRed: false,
	}
	dbStruct.Users[dbStruct.NextUserId] = user
	dbStruct.NextUserId++
	db.writeDB(dbStruct)
	return user, nil
}

func (db *DB) GetUser(email string) (User, error) {
	normalizedEmail := normalizeEmail(email)
	user, found, err := db.getUserByEmail(normalizedEmail)
	if err != nil {
		return User{}, err
	}
	if !found {
		return User{}, ErrUserDoesNotExist
	}
	return user, nil
}

func (db *DB) getUserByEmail(email string) (User, bool, error) {
	dbStruct, err := db.loadDB()
	if err != nil {
		return User{}, false, err
	}
	for id := range dbStruct.Users {
		user := dbStruct.Users[id]
		if email == user.Email {
			return user, true, nil
		}
	}
	return User{}, false, nil
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

func (db *DB) GetChirpsFromId(authorId int) ([]Chirp, error) {
	dbStruct, err := db.loadDB()
	if err != nil {
		return nil, err
	}
	keys := make([]Chirp, 0)
	i := 0
	for chirpId := range dbStruct.Chirps {
		chirp := dbStruct.Chirps[chirpId]
		if authorId == chirp.AuthorId {
			keys = append(keys, chirp)
		}
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
		NextChirpId:          1,
		NextUserId:           1,
		Chirps:               make(map[int]Chirp),
		Users:                make(map[int]User),
		RevokedRefreshTokens: make(map[string]time.Time),
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

func (db *DB) ComparePasswords(password, withEmail string) error {
	normalizedEmail := normalizeEmail(withEmail)
	user, err := db.GetUser(normalizedEmail)
	if err != nil {
		return err
	}
	err = bcrypt.CompareHashAndPassword(user.Password, []byte(password))
	return err
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

func (db *DB) UpdateUser(id int, email, password string) error {
	dbStruct, err := db.loadDB()
	if err != nil {
		return err
	}
	user, found := dbStruct.Users[id]
	if !found {
		return ErrUserDoesNotExist
	}
	hashPass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	updatedUser := User{
		Email:       email,
		Password:    hashPass,
		Id:          id,
		IsChirpyRed: user.IsChirpyRed,
	}
	dbStruct.Users[id] = updatedUser
	err = db.writeDB(dbStruct)
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) getUserIdByEmail(email string) (int, bool, error) {
	dbStruct, err := db.loadDB()
	if err != nil {
		return 0, false, err
	}
	for id := range dbStruct.Users {
		if email == dbStruct.Users[id].Email {
			return id, true, nil
		}
	}
	return 0, false, nil
}

func (db *DB) RevokeRefreshToken(token string) error {
	dbStruct, err := db.loadDB()
	if err != nil {
		return err
	}
	revoked, err := db.IsTokenRevoked(token)
	if err != nil {
		return err
	}
	if revoked {
		return ErrTokenAlreadyRevoked
	}
	dbStruct.RevokedRefreshTokens[token] = time.Now()
	if err := db.writeDB(dbStruct); err != nil {
		return err
	}
	return nil
}

func (db *DB) IsTokenRevoked(token string) (bool, error) {
	dbStruct, err := db.loadDB()
	if err != nil {
		return false, err
	}
	_, revoked := dbStruct.RevokedRefreshTokens[token]
	return revoked, nil
}

func (db *DB) UpgradeUser(id int) error {
	dbStruct, err := db.loadDB()
	if err != nil {
		return err
	}
	user, found := dbStruct.Users[id]
	if !found {
		return ErrUserDoesNotExist
	}
	user.IsChirpyRed = true
	dbStruct.Users[id] = user
	if err := db.writeDB(dbStruct); err != nil {
		return err
	}
	return nil
}
