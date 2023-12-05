package utils

import (
	"hash/fnv"
	"runtime"
	"sync"
)

// KeyMutex is a thread-safe interface for acquiring locks on arbitrary strings.
type KeyMutex interface {
	// Acquires a lock associated with the specified ID, creates the lock if one doesn't already exist.
	LockKey(id string)

	// Releases the lock associated with the specified ID.
	// Returns an error if the specified ID doesn't exist.
	UnlockKey(id string) error
}

// NewHashedKeyMutex returns a new instance of KeyMutex which hashes arbitrary keys to
// a fixed set of locks. `n` specifies number of locks, if n <= 0, we use
// number of cpus.
// Note that because it uses fixed set of locks, different keys may share same
// lock, so it's possible to wait on same lock.
func NewHashedKeyMutex(n int) KeyMutex {
	if n <= 0 {
		n = runtime.NumCPU()
	}
	return &hashedKeyMutex{
		mutexes: make([]sync.Mutex, n),
	}
}

type hashedKeyMutex struct {
	mutexes []sync.Mutex
}

// Acquires a lock associated with the specified ID.
func (km *hashedKeyMutex) LockKey(id string) {
	km.mutexes[km.hash(id)%uint32(len(km.mutexes))].Lock()
}

// Releases the lock associated with the specified ID.
func (km *hashedKeyMutex) UnlockKey(id string) error {
	km.mutexes[km.hash(id)%uint32(len(km.mutexes))].Unlock()
	return nil
}

func (km *hashedKeyMutex) hash(id string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return h.Sum32()
}
