package main

import (
	"testing"

	"ofam.dev/ochazuke"

	"ofam.dev/ochazuke/lexiconcheck"
)

// o-services's registered operations are held to the lexicon's naming rules
// (lexicon plan): one word has one part of speech, nouns are singular, a
// namespace is not a noun or action word, a key has at most three parts.
// testdata/lexicon-words.tsv is a copy of ochazuke's word list.
func TestLexicon(t *testing.T) {
	opened := []string{}
	f, _, _, _, _ := twoPortService()
	a, _ := testApp(t, f, noMetadata(t), &opened)
	lexiconcheck.Check(t, lexiconcheck.Config{
		Operations: &a.r.Operations,
		MainNouns:  map[string]string{"service": "service"},
		Words:      "testdata/lexicon-words.tsv",
		Baseline:   "testdata/lexiconcheck.txt",
	})
}

// The refresh verb acts on the whole service collection, not on a plural
// services noun.
func TestRefreshIsCollectionScope(t *testing.T) {
	opened := []string{}
	f, _, _, _, _ := twoPortService()
	a, _ := testApp(t, f, noMetadata(t), &opened)
	op, ok := a.r.Operations.Find(ochazuke.Key("service.service.refresh"))
	if !ok {
		op, ok = a.r.Operations.Find(ochazuke.Key("service.refresh"))
	}
	if !ok || op.Subject != ochazuke.SubjectCollection {
		t.Fatalf("service refresh: found %v, subject %q", ok, op.Subject)
	}
	if _, old := a.r.Operations.Find(ochazuke.Key("services.refresh")); old {
		t.Fatal("services.refresh is still registered")
	}
}
