package database

import (
	"os"
	"sync"
	"testing"
)

func Test(t *testing.T) {
	runExistsTest(t, "./database.go", true)
	runExistsTest(t, "./this/does/not/exists.go", false)

	runEnsureDBTest(t)

	runGetChirpsTest(t)
}

func runExistsTest(t *testing.T, path string, expecting bool) {
	t.Logf("Starting test for exists with: \"%s\", and expecting: %t", path, expecting)
	got := exists(path)
	if got != expecting {
		t.Errorf("Expecting: %t, but got: %t", expecting, got)
	}
}

func runEnsureDBTest(t *testing.T) {
	path := "./test_db.gob"
	defer os.Remove(path)
	db := &DB{
		path:   path,
		mux:    &sync.RWMutex{},
		nextId: 0,
	}
	t.Logf("Starting test for ensureDB when DB does not exist with: \"%s\", and expecting: true", path)
	err := db.ensureDB()
	if err != nil {
		t.Error(err)
	}
	got := exists(path)
	if got != true {
		t.Errorf("Expecting: true, but got: %t", got)
	}
	t.Logf("Starting test for ensureDB when DB does exist with: \"%s\", and expecting: true", path)
	err = db.ensureDB()
	if err != nil {
		t.Error(err)
	}
	got = exists(path)
	if got != true {
		t.Errorf("Expecting: true, but got: %t", got)
	}
}

func runGetChirpsTest(t *testing.T) {
	path := "./test_db.gob"
	defer os.Remove(path)

	expecting := []Chirp{
		{Id: 0, Body: "Some chirp"},
		{Id: 1, Body: "Some other chirp"},
	}

	t.Logf("Starting test for ensureDB when DB does exist with: \"%s\", and expecting: %v", path, expecting)

	db, err := NewDB(path)
	if err != nil {
		t.Error(err)
	}

	dbStruct := DBStructure{Chirps: make(map[int]Chirp)}
	dbStruct.Chirps[0] = Chirp{Id: 0, Body: "Some chirp"}
	dbStruct.Chirps[1] = Chirp{Id: 1, Body: "Some other chirp"}

	err = db.writeDB(dbStruct)
	if err != nil {
		t.Error(err)
	}

	got, err := db.GetChirps()
	if err != nil {
		t.Error(err)
	}

	for i, _ := range got {
		if got[i] != expecting[i] {
			t.Errorf("Expecting: %v, but got: %v", expecting, got)
		}
	}

}
