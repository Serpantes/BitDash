package repository

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"
)

const storageCapacityFile = "storage-capacity.json"

// StorageCapacity serves the item and cargo pocket counts of every storage building and
// deployable by name, so inventories can show their empty pockets. It is built from BitJita's
// /buildings (~1.8 MB) and /deployables listings, kept on disk and rebuilt once it is a day old.
type StorageCapacity struct {
	bitjita *BitjitaCache
	maxAge  time.Duration

	mu       sync.Mutex
	body     []byte
	builtAt  time.Time
	building bool
}

type storageCapacityPayload struct {
	BuiltAt  int64             `json:"builtAt"`
	Storages map[string][2]int `json:"storages"`
}

func NewStorageCapacity(bitjita *BitjitaCache, maxAge time.Duration) *StorageCapacity {
	s := &StorageCapacity{bitjita: bitjita, maxAge: maxAge}
	if data, ok := bitjita.apiFiles.Get(storageCapacityFile); ok {
		var payload storageCapacityPayload
		if json.Unmarshal(data, &payload) == nil && len(payload.Storages) > 0 {
			s.body, s.builtAt = data, time.Unix(payload.BuiltAt, 0)
		}
	}
	return s
}

// Get returns the stored map at once and rebuilds it in the background when old;
// only the very first call waits for BitJita.
func (s *StorageCapacity) Get() ([]byte, error) {
	s.mu.Lock()
	body, age := s.body, time.Since(s.builtAt)
	if body != nil && age > s.maxAge && !s.building {
		s.building = true
		go func() {
			if err := s.rebuild(); err != nil {
				log.Printf("storage capacity: %v", err)
			}
		}()
	}
	s.mu.Unlock()
	if body != nil {
		return body, nil
	}
	if err := s.rebuild(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.body, nil
}

func (s *StorageCapacity) rebuild() error {
	defer func() {
		s.mu.Lock()
		s.building = false
		s.mu.Unlock()
	}()
	buildings, _, _, err := s.bitjita.GetAPI("/buildings", "")
	if err != nil {
		return err
	}
	deployables, _, _, err := s.bitjita.GetAPI("/deployables", "")
	if err != nil {
		return err
	}
	now := time.Now()
	body, err := buildStorageCapacity(buildings, deployables, now)
	if err != nil {
		return err
	}
	_ = s.bitjita.apiFiles.Set(storageCapacityFile, body)
	s.mu.Lock()
	s.body, s.builtAt = body, now
	s.mu.Unlock()
	return nil
}

func buildStorageCapacity(buildings, deployables []byte, builtAt time.Time) ([]byte, error) {
	var buildingRows []struct {
		Name      string `json:"name"`
		Functions []struct {
			StorageSlots int `json:"storage_slots"`
			CargoSlots   int `json:"cargo_slots"`
		} `json:"functions"`
	}
	if err := json.Unmarshal(buildings, &buildingRows); err != nil {
		return nil, err
	}
	var deployableRows struct {
		Deployables []struct {
			Name      string `json:"name"`
			Storage   int    `json:"storage"`
			Stockpile int    `json:"stockpile"`
		} `json:"deployables"`
	}
	if err := json.Unmarshal(deployables, &deployableRows); err != nil {
		return nil, err
	}

	storages := map[string][2]int{}
	for _, b := range buildingRows {
		var items, cargo int
		for _, f := range b.Functions {
			items += f.StorageSlots
			cargo += f.CargoSlots
		}
		if b.Name != "" && (items > 0 || cargo > 0) {
			storages[b.Name] = [2]int{items, cargo}
		}
	}
	for _, d := range deployableRows.Deployables {
		if d.Name != "" && (d.Storage > 0 || d.Stockpile > 0) {
			storages[d.Name] = [2]int{d.Storage, d.Stockpile}
		}
	}
	if len(storages) == 0 {
		return nil, errors.New("no storages in BitJita listings")
	}
	return json.Marshal(storageCapacityPayload{BuiltAt: builtAt.Unix(), Storages: storages})
}
