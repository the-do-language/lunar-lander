package db

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
)

type Record map[string]any

type Store struct {
	mu     sync.Mutex
	path   string
	data   map[string][]Record
	nextID int
}

func Open(path string) (*Store, error) {
	store := &Store{
		path:   path,
		data:   map[string][]Record{},
		nextID: 1,
	}
	if path == "" {
		return store, nil
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return nil, err
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return store, nil
	}
	if err := json.Unmarshal(payload, &store.data); err != nil {
		return nil, err
	}
	store.nextID = store.maxID() + 1
	return store, nil
}

func (s *Store) Query(collection string, criteria Record) ([]Record, error) {
	if collection == "" {
		return nil, errors.New("collection is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	records := s.data[collection]
	result := make([]Record, 0)
	for _, record := range records {
		if matches(record, criteria) {
			result = append(result, cloneRecord(record))
		}
	}
	return result, nil
}

func (s *Store) Insert(collection string, record Record) (Record, error) {
	if collection == "" {
		return nil, errors.New("collection is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if record == nil {
		record = Record{}
	}
	copy := cloneRecord(record)
	if _, ok := copy["id"]; !ok {
		copy["id"] = s.nextID
		s.nextID++
	}
	s.data[collection] = append(s.data[collection], copy)
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return cloneRecord(copy), nil
}

func (s *Store) Update(collection string, criteria Record, updates Record) (int, error) {
	if collection == "" {
		return 0, errors.New("collection is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	records := s.data[collection]
	updated := 0
	for i, record := range records {
		if !matches(record, criteria) {
			continue
		}
		for key, value := range updates {
			record[key] = value
		}
		records[i] = record
		updated++
	}
	s.data[collection] = records
	if err := s.saveLocked(); err != nil {
		return 0, err
	}
	return updated, nil
}

func (s *Store) Delete(collection string, criteria Record) (int, error) {
	if collection == "" {
		return 0, errors.New("collection is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	records := s.data[collection]
	remaining := records[:0]
	deleted := 0
	for _, record := range records {
		if matches(record, criteria) {
			deleted++
			continue
		}
		remaining = append(remaining, record)
	}
	s.data[collection] = remaining
	if err := s.saveLocked(); err != nil {
		return 0, err
	}
	return deleted, nil
}

func (s *Store) saveLocked() error {
	if s.path == "" {
		return nil
	}
	dir := filepath.Dir(s.path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	payload, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, payload, 0o600)
}

func (s *Store) maxID() int {
	maxID := 0
	for _, records := range s.data {
		for _, record := range records {
			if value, ok := numberValue(record["id"]); ok {
				if int(value) > maxID {
					maxID = int(value)
				}
			}
		}
	}
	return maxID
}

func cloneRecord(record Record) Record {
	copy := make(Record, len(record))
	for key, value := range record {
		copy[key] = value
	}
	return copy
}

func matches(record Record, criteria Record) bool {
	if len(criteria) == 0 {
		return true
	}
	for key, expected := range criteria {
		actual, ok := record[key]
		if !ok {
			return false
		}
		if !valuesEqual(actual, expected) {
			return false
		}
	}
	return true
}

func valuesEqual(left, right any) bool {
	leftNumber, leftIsNumber := numberValue(left)
	rightNumber, rightIsNumber := numberValue(right)
	if leftIsNumber && rightIsNumber {
		return leftNumber == rightNumber
	}
	return reflect.DeepEqual(left, right)
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
