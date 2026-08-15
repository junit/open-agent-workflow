package rollout

import (
	"fmt"
	"hash/fnv"
)

// Bucket returns the stable rollout bucket for a subject key.
func Bucket(key string) uint32 {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	return hash.Sum32() % 100
}

// Select returns keys whose stable bucket falls within percentage.
func Select(percentage int, keys []string) ([]string, error) {
	if percentage < 0 || percentage > 100 {
		return nil, fmt.Errorf("percentage must be between 0 and 100")
	}
	for _, key := range keys {
		if key == "" {
			return nil, fmt.Errorf("key must not be empty")
		}
	}

	selected := make([]string, 0, len(keys))
	for _, key := range keys {
		if Bucket(key) < uint32(percentage) {
			selected = append(selected, key)
		}
	}
	return selected, nil
}
