package session

import "errors"

var ErrUnknownSession = errors.New("unknown session")

type Record struct {
	Token        Token
	RequesterUID int
}

type Store struct {
	records map[Token]Record
}

func NewStore() *Store {
	return &Store{records: map[Token]Record{}}
}

func (store *Store) Create(token Token, requesterUID int) Record {
	record := Record{Token: token, RequesterUID: requesterUID}
	store.records[token] = record
	return record
}

func (store *Store) Lookup(token Token) (Record, bool) {
	record, ok := store.records[token]
	return record, ok
}

func (store *Store) Require(token Token) (Record, error) {
	record, ok := store.Lookup(token)
	if !ok {
		return Record{}, ErrUnknownSession
	}
	return record, nil
}

func (store *Store) Destroy(token Token) bool {
	if _, ok := store.records[token]; !ok {
		return false
	}
	delete(store.records, token)
	return true
}

