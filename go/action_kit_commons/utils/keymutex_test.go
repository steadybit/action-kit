package utils

import (
	"testing"
	"time"
)

const (
	callbackTimeout = 1 * time.Second
)

func newKeyMutexes() []KeyMutex {
	return []KeyMutex{
		NewHashedKeyMutex(0),
		NewHashedKeyMutex(1),
		NewHashedKeyMutex(2),
		NewHashedKeyMutex(4),
	}
}

func Test_SingleLock_NoUnlock(t *testing.T) {
	for _, km := range newKeyMutexes() {
		// Arrange
		key := "fakeid"
		callbackCh := make(chan interface{})

		// Act
		go lockAndCallback(km, key, callbackCh)

		// Assert
		verifyCallbackHappens(t, callbackCh)
	}
}

func Test_SingleLock_SingleUnlock(t *testing.T) {
	for _, km := range newKeyMutexes() {
		// Arrange
		key := "fakeid"
		callbackCh := make(chan interface{})

		// Act & Assert
		go lockAndCallback(km, key, callbackCh)
		verifyCallbackHappens(t, callbackCh)
		_ = km.UnlockKey(key)
	}
}

func Test_DoubleLock_DoubleUnlock(t *testing.T) {
	for _, km := range newKeyMutexes() {
		// Arrange
		key := "fakeid"
		callbackCh1stLock := make(chan interface{})
		callbackCh2ndLock := make(chan interface{})

		// Act & Assert
		go lockAndCallback(km, key, callbackCh1stLock)
		verifyCallbackHappens(t, callbackCh1stLock)
		go lockAndCallback(km, key, callbackCh2ndLock)
		verifyCallbackDoesntHappens(t, callbackCh2ndLock)
		_ = km.UnlockKey(key)
		verifyCallbackHappens(t, callbackCh2ndLock)
		_ = km.UnlockKey(key)
	}
}

func lockAndCallback(km KeyMutex, id string, callbackCh chan<- interface{}) {
	km.LockKey(id)
	callbackCh <- true
}

func verifyCallbackHappens(t *testing.T, callbackCh <-chan interface{}) bool {
	select {
	case <-callbackCh:
		return true
	case <-time.After(callbackTimeout):
		t.Fatalf("Timed out waiting for callback.")
		return false
	}
}

func verifyCallbackDoesntHappens(t *testing.T, callbackCh <-chan interface{}) bool {
	select {
	case <-callbackCh:
		t.Fatalf("Unexpected callback.")
		return false
	case <-time.After(callbackTimeout):
		return true
	}
}
