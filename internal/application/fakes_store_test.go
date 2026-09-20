package application

import (
	"context"
	"strings"
	"sync"

	"github.com/oernster/ScreenState/internal/domain"
)

// fakeStore is a profile store in memory, atomic by being a map.
type fakeStore struct {
	mutex      sync.Mutex
	profiles   map[string]domain.Profile
	order      []string
	namesErr   error
	loadErr    error
	saveErr    error
	deleteErr  error
	defaultErr error
}

func newFakeStore(profiles ...domain.Profile) *fakeStore {
	store := &fakeStore{profiles: make(map[string]domain.Profile)}
	for _, profile := range profiles {
		store.profiles[strings.ToLower(profile.Name)] = profile
		store.order = append(store.order, profile.Name)
	}
	return store
}

func (store *fakeStore) Names(context.Context) ([]string, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.namesErr != nil {
		return nil, store.namesErr
	}
	copied := make([]string, len(store.order))
	copy(copied, store.order)
	return copied, nil
}

func (store *fakeStore) Load(_ context.Context, name string) (domain.Profile, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.loadErr != nil {
		return domain.Profile{}, store.loadErr
	}
	profile, held := store.profiles[strings.ToLower(name)]
	if !held {
		return domain.Profile{}, ErrNoSuchProfile
	}
	return profile, nil
}

func (store *fakeStore) Save(_ context.Context, profile domain.Profile) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.saveErr != nil {
		return store.saveErr
	}
	key := strings.ToLower(profile.Name)
	if _, held := store.profiles[key]; !held {
		store.order = append(store.order, profile.Name)
	}
	store.profiles[key] = profile
	return nil
}

func (store *fakeStore) Delete(_ context.Context, name string) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.deleteErr != nil {
		return store.deleteErr
	}
	key := strings.ToLower(name)
	if _, held := store.profiles[key]; !held {
		return ErrNoSuchProfile
	}
	delete(store.profiles, key)
	for at, stored := range store.order {
		if strings.EqualFold(stored, name) {
			store.order = append(store.order[:at], store.order[at+1:]...)
			break
		}
	}
	return nil
}

func (store *fakeStore) Default(context.Context) (domain.Profile, bool, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.defaultErr != nil {
		return domain.Profile{}, false, store.defaultErr
	}
	for _, name := range store.order {
		profile := store.profiles[strings.ToLower(name)]
		if profile.Default {
			return profile, true, nil
		}
	}
	return domain.Profile{}, false, nil
}
