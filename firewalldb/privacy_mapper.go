package firewalldb

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"

	"github.com/lightninglabs/lightning-terminal/session"
	"go.etcd.io/bbolt"
)

/*
	The PrivacyMapper data is stored in the following structure in the db:

	privacy -> group id -> real-to-pseudo -> {k:v}
			    -> pseudo-to-real -> {k:v}
*/

const (
	txidStringLen = 64
)

var (
	privacyBucketKey = []byte("privacy")
	realToPseudoKey  = []byte("real-to-pseudo")
	pseudoToRealKey  = []byte("pseudo-to-real")

	pseudoStrAlphabet    = []rune("abcdef0123456789")
	pseudoStrAlphabetLen = len(pseudoStrAlphabet)
)

// NewPrivacyMapDB is a function type that takes a group ID and uses it to
// construct a new PrivacyMapDB.
type NewPrivacyMapDB func(groupID session.ID) PrivacyMapDB

// PrivacyDB constructs a PrivacyMapDB that will be indexed under the given
// group ID key.
func (db *BoltDB) PrivacyDB(groupID session.ID) PrivacyMapDB {
	return &kvdbExecutor[PrivacyMapTx]{
		db: db.DB,
		wrapTx: func(tx *bbolt.Tx) PrivacyMapTx {
			return &privacyMapTx{
				boltTx: tx,
				privacyMapDB: &privacyMapDB{
					groupID: groupID,
				},
			}
		},
	}
}

// PrivacyMapDB provides an Update and View method that will allow the caller
// to perform atomic read and write transactions defined by PrivacyMapTx on the
// underlying DB.
type PrivacyMapDB = DBExecutor[PrivacyMapTx]

// PrivacyMapTx represents a db that can be used to create, store and fetch
// real-pseudo pairs.
type PrivacyMapTx interface {
	// NewPair persists a new real-pseudo pair.
	NewPair(ctx context.Context, real, pseudo string) error

	// PseudoToReal returns the real value associated with the given pseudo
	// value. If no such pair is found, then ErrNoSuchKeyFound is returned.
	PseudoToReal(ctx context.Context, pseudo string) (string, error)

	// RealToPseudo returns the pseudo value associated with the given real
	// value. If no such pair is found, then ErrNoSuchKeyFound is returned.
	RealToPseudo(ctx context.Context, real string) (string, error)

	// FetchAllPairs loads and returns the real-to-pseudo pairs in the form
	// of a PrivacyMapPairs struct.
	FetchAllPairs(ctx context.Context) (*PrivacyMapPairs, error)
}

// privacyMapDB is an implementation of PrivacyMapDB.
type privacyMapDB struct {
	groupID session.ID
}

// privacyMapTx is an implementation of PrivacyMapTx.
type privacyMapTx struct {
	*privacyMapDB
	boltTx *bbolt.Tx
}

// NewPair inserts a new real-pseudo pair into the db.
//
// NOTE: this is part of the PrivacyMapTx interface.
func (p *privacyMapTx) NewPair(_ context.Context, real, pseudo string) error {
	privacyBucket, err := getBucket(p.boltTx, privacyBucketKey)
	if err != nil {
		return err
	}

	sessBucket, err := privacyBucket.CreateBucketIfNotExists(p.groupID[:])
	if err != nil {
		return err
	}

	realToPseudoBucket, err := sessBucket.CreateBucketIfNotExists(
		realToPseudoKey,
	)
	if err != nil {
		return err
	}

	pseudoToRealBucket, err := sessBucket.CreateBucketIfNotExists(
		pseudoToRealKey,
	)
	if err != nil {
		return err
	}

	if len(realToPseudoBucket.Get([]byte(real))) != 0 {
		return fmt.Errorf("an entry already exists for real "+
			"value: %x", real)
	}

	if len(pseudoToRealBucket.Get([]byte(pseudo))) != 0 {
		return fmt.Errorf("an entry already exists for pseudo "+
			"value: %x", pseudo)
	}

	err = realToPseudoBucket.Put([]byte(real), []byte(pseudo))
	if err != nil {
		return err
	}

	return pseudoToRealBucket.Put([]byte(pseudo), []byte(real))
}

// PseudoToReal will check the db to see if the given pseudo key exists. If
// it does then the real value is returned, else an error is returned.
//
// NOTE: this is part of the PrivacyMapTx interface.
func (p *privacyMapTx) PseudoToReal(_ context.Context, pseudo string) (string,
	error) {

	privacyBucket, err := getBucket(p.boltTx, privacyBucketKey)
	if err != nil {
		return "", err
	}

	sessBucket := privacyBucket.Bucket(p.groupID[:])
	if sessBucket == nil {
		return "", ErrNoSuchKeyFound
	}

	pseudoToRealBucket := sessBucket.Bucket(pseudoToRealKey)
	if pseudoToRealBucket == nil {
		return "", ErrNoSuchKeyFound
	}

	real := pseudoToRealBucket.Get([]byte(pseudo))
	if len(real) == 0 {
		return "", ErrNoSuchKeyFound
	}

	return string(real), nil
}

// RealToPseudo will check the db to see if the given real key exists. If
// it does then the pseudo value is returned, else an error is returned.
//
// NOTE: this is part of the PrivacyMapTx interface.
func (p *privacyMapTx) RealToPseudo(_ context.Context, real string) (string,
	error) {

	privacyBucket, err := getBucket(p.boltTx, privacyBucketKey)
	if err != nil {
		return "", err
	}

	sessBucket := privacyBucket.Bucket(p.groupID[:])
	if sessBucket == nil {
		return "", ErrNoSuchKeyFound
	}

	realToPseudoBucket := sessBucket.Bucket(realToPseudoKey)
	if realToPseudoBucket == nil {
		return "", ErrNoSuchKeyFound
	}

	pseudo := realToPseudoBucket.Get([]byte(real))
	if len(pseudo) == 0 {
		return "", ErrNoSuchKeyFound
	}

	return string(pseudo), nil
}

// FetchAllPairs loads and returns the real-to-pseudo pairs.
//
// NOTE: this is part of the PrivacyMapTx interface.
func (p *privacyMapTx) FetchAllPairs(_ context.Context) (*PrivacyMapPairs,
	error) {

	privacyBucket, err := getBucket(p.boltTx, privacyBucketKey)
	if err != nil {
		return nil, err
	}

	sessBucket := privacyBucket.Bucket(p.groupID[:])
	if sessBucket == nil {
		// If the bucket has not been created yet, then there are no
		// privacy pairs yet.
		return NewPrivacyMapPairs(nil), nil
	}

	realToPseudoBucket := sessBucket.Bucket(realToPseudoKey)
	if realToPseudoBucket == nil {
		return nil, ErrNoSuchKeyFound
	}

	pairs := make(map[string]string)
	err = realToPseudoBucket.ForEach(func(r, p []byte) error {
		pairs[string(r)] = string(p)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return NewPrivacyMapPairs(pairs), nil
}

func HideString(ctx context.Context, tx PrivacyMapTx, real string) (string,
	error) {

	pseudo, err := tx.RealToPseudo(ctx, real)
	if err != nil && err != ErrNoSuchKeyFound {
		return "", err
	}
	if err == nil {
		return pseudo, nil
	}

	pseudo, err = NewPseudoStr(len(real))
	if err != nil {
		return "", err
	}

	if err = tx.NewPair(ctx, real, pseudo); err != nil {
		return "", err
	}

	return pseudo, nil
}

func NewPseudoStr(n int) (string, error) {
	var max big.Int
	max.SetUint64(uint64(pseudoStrAlphabetLen))

	b := make([]rune, n)
	for i := range b {
		index, err := rand.Int(rand.Reader, &max)
		if err != nil {
			return "", err
		}

		b[i] = pseudoStrAlphabet[index.Uint64()]
	}

	return string(b), nil
}

func RevealString(ctx context.Context, tx PrivacyMapTx, pseudo string) (string,
	error) {

	if pseudo == "" {
		return pseudo, nil
	}

	return tx.PseudoToReal(ctx, pseudo)
}

func HideUint64(ctx context.Context, tx PrivacyMapTx, real uint64) (uint64,
	error) {

	str := Uint64ToStr(real)
	pseudo, err := tx.RealToPseudo(ctx, str)
	if err != nil && err != ErrNoSuchKeyFound {
		return 0, err
	}
	if err == nil {
		return StrToUint64(pseudo)
	}

	pseudoUint64, pseudoUint64Str := NewPseudoUint64()
	if err := tx.NewPair(ctx, str, pseudoUint64Str); err != nil {
		return 0, err
	}

	return pseudoUint64, nil
}

func RevealUint64(ctx context.Context, tx PrivacyMapTx, pseudo uint64) (uint64,
	error) {

	if pseudo == 0 {
		return 0, nil
	}

	real, err := tx.PseudoToReal(ctx, Uint64ToStr(pseudo))
	if err != nil {
		return 0, err
	}

	return StrToUint64(real)
}

func HideChanPoint(ctx context.Context, tx PrivacyMapTx, txid string,
	index uint32) (string, uint32, error) {

	cp := fmt.Sprintf("%s:%d", txid, index)
	pseudo, err := tx.RealToPseudo(ctx, cp)
	if err != nil && err != ErrNoSuchKeyFound {
		return "", 0, err
	}
	if err == nil {
		return DecodeChannelPoint(pseudo)
	}

	newCp, err := NewPseudoChanPoint()
	if err != nil {
		return "", 0, err
	}

	if err := tx.NewPair(ctx, cp, newCp); err != nil {
		return "", 0, err
	}

	return DecodeChannelPoint(newCp)
}

func NewPseudoChanPoint() (string, error) {
	pseudoTXID, err := NewPseudoStr(txidStringLen)
	if err != nil {
		return "", err
	}

	pseudoIndex := NewPseudoUint32()
	return fmt.Sprintf("%s:%d", pseudoTXID, pseudoIndex), nil
}

func RevealChanPoint(ctx context.Context, tx PrivacyMapTx, txid string,
	index uint32) (string, uint32, error) {

	fakePoint := fmt.Sprintf("%s:%d", txid, index)
	real, err := tx.PseudoToReal(ctx, fakePoint)
	if err != nil {
		return "", 0, err
	}

	return DecodeChannelPoint(real)
}

func NewPseudoUint32() uint32 {
	b := make([]byte, 4)
	_, _ = rand.Read(b)

	return binary.BigEndian.Uint32(b)
}

func HideChanPointStr(ctx context.Context, tx PrivacyMapTx, cp string) (string,
	error) {

	txid, index, err := DecodeChannelPoint(cp)
	if err != nil {
		return "", err
	}

	newTxid, newIndex, err := HideChanPoint(ctx, tx, txid, index)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%d", newTxid, newIndex), nil
}

func HideBytes(ctx context.Context, tx PrivacyMapTx, realBytes []byte) ([]byte,
	error) {

	real := hex.EncodeToString(realBytes)

	pseudo, err := HideString(ctx, tx, real)
	if err != nil {
		return nil, err
	}

	return hex.DecodeString(pseudo)
}

func RevealBytes(ctx context.Context, tx PrivacyMapTx,
	pseudoBytes []byte) ([]byte, error) {

	if pseudoBytes == nil {
		return nil, nil
	}

	pseudo := hex.EncodeToString(pseudoBytes)
	pseudo, err := RevealString(ctx, tx, pseudo)
	if err != nil {
		return nil, err
	}

	return hex.DecodeString(pseudo)
}

func NewPseudoUint64() (uint64, string) {
	b := make([]byte, 8)
	_, _ = rand.Read(b)

	i := binary.BigEndian.Uint64(b)

	return i, hex.EncodeToString(b)
}

func Uint64ToStr(i uint64) string {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, i)
	return hex.EncodeToString(b)
}

func StrToUint64(s string) (uint64, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint64(b), nil
}

func DecodeChannelPoint(cp string) (string, uint32, error) {
	parts := strings.Split(cp, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("bad channel point encoding")
	}

	index, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", 0, err
	}

	if len(parts[0]) != txidStringLen {
		return "", 0, fmt.Errorf("wrong txid length want %v, got %v",
			txidStringLen, len(parts[0]))
	}

	return parts[0], uint32(index), nil
}

// PrivacyMapReader is an interface that gives read access to a privacy map
// DB.
type PrivacyMapReader interface {
	// GetPseudo returns the associated pseudo value for a given real value.
	// If no such real value exists in the DB, then false is returned.
	GetPseudo(real string) (string, bool)
}

// PrivacyMapPairs is an in memory implementation of the PrivacyMapReader.
type PrivacyMapPairs struct {
	// pairs is a map from real to pseudo strings.
	pairs map[string]string

	mu sync.Mutex
}

// NewPrivacyMapPairs constructs a new PrivacyMapPairs struct. It may be
// initialised with either a nil map or a pre-defined real-to-pseudo strings
// map.
func NewPrivacyMapPairs(m map[string]string) *PrivacyMapPairs {
	if m != nil {
		return &PrivacyMapPairs{
			pairs: m,
		}
	}

	return &PrivacyMapPairs{
		pairs: make(map[string]string),
	}
}

// GetPseudo returns the associated pseudo value for a given real value. If no
// such real value exists in the DB, then false is returned.
//
// NOTE: this is part of the PrivacyMapReader interface.
func (p *PrivacyMapPairs) GetPseudo(real string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	pseudo, ok := p.pairs[real]

	return pseudo, ok
}

// Add adds the passed set of real-to-pseudo pairs to the PrivacyMapPairs
// structure. It will throw an error if the new pairs conflict with any of the
// existing pairs.
func (p *PrivacyMapPairs) Add(pairs map[string]string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Do a first pass to ensure that none of the new entries conflict with
	// the existing entries. We do this so that we don't mutate the set of
	// pairs before we know that the new set is valid.
	for realStr, pseudoStr := range pairs {
		ps, ok := p.pairs[realStr]
		if ok && ps != pseudoStr {
			return fmt.Errorf("cannot replace existing pseudo "+
				"entry for real value: %s", realStr)
		}
	}

	// In our second pass, we can add the new pairs to our set.
	for realStr, pseudoStr := range pairs {
		p.pairs[realStr] = pseudoStr
	}

	return nil
}
