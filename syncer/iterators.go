package syncer

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
	"powerdns.com/platform/lightningstream/snapshot"
)

// ErrNoTimestamp is returned when an entry does not contain a timestamp, or the
// timestamp is 0.
var ErrNoTimestamp = errors.New("no timestamp for entry")

// TimestampedIterator iterates over a snapshot DBI and updates the LMDB with
// values that are prefixed with a timestamp header.
// This iterator has two uses:
// * Merge the main database into a shadow database with a default timestamp.
// * Merge a remote snapshot with the timestamp values into a DBI with timestamps.
// The LMDB values the iterator operates MUST always have a timestamp. If no
// timestamp is present (or it is 0), an error is returned.
type TimestampedIterator struct {
	Entries              []snapshot.KV // LMDB contents as raw values without timestamp
	DefaultTimestampNano uint64        // Timestamp to add to entries that do not have one

	current int
	started bool
	buf     []byte
}

func (it *TimestampedIterator) Next() (key []byte, err error) {
	if it.started {
		it.current++
	} else {
		it.started = true
	}
	if len(it.Entries) <= it.current {
		return nil, io.EOF
	}
	key = it.Entries[it.current].Key
	return key, nil
}

// Merge compares the old LMDB value currently stored and the current iterator
// value from the dump, and decides which value the LMDB should take.
// The LMDB entries are always prefixed with a big endian 64 bit timestamp.
func (it *TimestampedIterator) Merge(oldval []byte) (val []byte, err error) {
	entry := it.Entries[it.current]
	entryVal := entry.Value
	if len(oldval) == 0 {
		// Not in destination db, add with timestamp
		return it.addTS(entryVal, entry.TimestampNano)
	}
	if len(oldval) < HeaderSize {
		// Should never happen
		it.logDebugValue(oldval)
		return nil, fmt.Errorf("merge: oldval in db too short: %v = %v", entry.Key, oldval)
	}
	oldTS := binary.BigEndian.Uint64(oldval[:HeaderSize])
	newTS := entry.TimestampNano
	actualOldVal := oldval[HeaderSize:]
	if newTS == 0 {
		// Special handling for main to shadow copy that uses a default timestamp
		if bytes.Equal(actualOldVal, entryVal) {
			return oldval, nil // do not update timestamp
		}
		newTS = it.DefaultTimestampNano
	}
	if newTS < oldTS {
		// Current LMDB value has a higher timestamp, so keep that one
		return oldval, nil
	}
	if newTS == oldTS && bytes.Compare(actualOldVal, entryVal) <= 0 {
		// Same timestamp, lexicographic lower value wins for deterministic values,
		// so return the old value if the plain value was lower or equal.
		return oldval, nil
	}
	// Update LMDB value
	return it.addTS(entryVal, newTS)
}

func (it *TimestampedIterator) Clean(oldval []byte) (val []byte, err error) {
	if len(oldval) == HeaderSize {
		return oldval, nil // already deleted, only timestamp
	}
	return it.addTS(nil, 0)
}

func (it *TimestampedIterator) logDebugValue(val []byte) {
	entry := it.Entries[it.current]
	logrus.WithFields(logrus.Fields{
		"key": hex.Dump(entry.Key),
		"val": hex.Dump(val),
	}).Debug("LMDB value dump")
}

// addTS prepends a timestamp header to a plain value. It uses the ts parameter
// passed in if non-zero, or the default one set on the iterator.
// A timestamp is mandatory. If both are 0, an ErrNoTimestamp error is returned.
func (it *TimestampedIterator) addTS(entryVal []byte, ts uint64) (val []byte, err error) {
	if cap(it.buf) < HeaderSize {
		it.buf = make([]byte, HeaderSize, 1024)
	} else {
		it.buf = it.buf[:HeaderSize]
	}
	if ts == 0 {
		ts = it.DefaultTimestampNano
		if ts == 0 {
			return nil, ErrNoTimestamp
		}
	}
	binary.BigEndian.PutUint64(it.buf, ts)
	it.buf = append(it.buf, entryVal...)
	val = it.buf
	return val, nil
}

// PlainIterator iterates over a snapshot of a shadow database for
// insertion into the main database without the timestamp header.
type PlainIterator struct {
	Entries []snapshot.KV // LMDB contents with timestamp to merge

	current int
	started bool
}

func (it *PlainIterator) Next() (key []byte, err error) {
	if it.started {
		it.current++
	} else {
		it.started = true
	}
	if len(it.Entries) <= it.current {
		return nil, io.EOF
	}
	key = it.Entries[it.current].Key
	return key, nil
}

func (it *PlainIterator) Merge(oldval []byte) (val []byte, err error) {
	mainVal := it.Entries[it.current].Value
	return mainVal, nil
}

func (it *PlainIterator) Clean(oldval []byte) (val []byte, err error) {
	return nil, nil // Delete the key
}