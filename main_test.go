package main

import (
	"testing"
)

func Test(t *testing.T) {
	runCleanChirpTest(t, "This kerfuffle is crazy!", "This **** is crazy!")
	runCleanChirpTest(t, "Oh sharbert", "Oh ****")
	runCleanChirpTest(t, "FORNAX THAT!", "**** THAT!")
	runCleanChirpTest(t, "keRFuffle shARBert FORNax", "**** **** ****")
	runCleanChirpTest(t, "My mama taught me not to curse", "My mama taught me not to curse")
}

func runCleanChirpTest(t *testing.T, base, expecting string) {
	t.Logf("Starting test for cleanChirp with: \"%s\", and expecting: \"%s\"", base, expecting)
	got := cleanChirp(base)
	if got != expecting {
		t.Errorf("Expecting: %s, but got: %s", expecting, got)
	}
}
