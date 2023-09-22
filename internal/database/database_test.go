package database

import "testing"

func Test(t *testing.T) {
	runExistsTest(t, "./database.go", true)
	runExistsTest(t, "./this/does/not/exists.go", false)
}

func runExistsTest(t *testing.T, path string, expecting bool) {
	t.Logf("Starting test for exists with: \"%s\", and expecting: %t", path, expecting)
	got := exists(path)
	if got != expecting {
		t.Errorf("Expecting: %t, but got: %t", expecting, got)
	}
}
