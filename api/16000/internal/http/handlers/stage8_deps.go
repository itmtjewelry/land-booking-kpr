package handlers

import (
	"sync"

	"github.com/itmtjewelry/land-booking-kpr/internal/storage"
)

type Stage8Deps interface {
	Stage7Deps

	StorageDir() string
	Loaded() map[string]storage.JSONFile
	LockForFile(filename string) *sync.Mutex
	ReloadCore() error
}
