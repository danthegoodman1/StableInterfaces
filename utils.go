package stableinterfaces

import (
	"crypto/sha256"
	"fmt"
	"os"
	"regexp"
	"strconv"
)

func getEnvOrDefault(env, defaultVal string) string {
	e := os.Getenv(env)
	if e == "" {
		return defaultVal
	} else {
		return e
	}
}

func getEnvOrDefaultInt(env string, defaultVal int64) int64 {
	e := os.Getenv(env)
	if e == "" {
		return defaultVal
	} else {
		intVal, err := strconv.ParseInt(e, 10, 16)
		if err != nil {
			return 0
		}

		return intVal
	}
}

// Entirely gpt-4 generated, seems to work!
func expandRangePattern(input string) ([]string, error) {
	// Find all matches for the pattern `{number..number}`
	re := regexp.MustCompile(`\{(\d+)\.\.(\d+)\}`)
	matches := re.FindAllStringSubmatch(input, -1)

	// If no matches, return the input as the only element in the slice
	if len(matches) == 0 {
		return []string{input}, nil
	}

	// Assuming we only handle the first range for simplicity
	// Further logic could be added to handle multiple ranges
	for _, match := range matches {
		start, err := strconv.Atoi(match[1])
		if err != nil {
			return nil, err
		}
		end, err := strconv.Atoi(match[2])
		if err != nil {
			return nil, err
		}

		// Generate the expanded strings
		var expandedStrings []string
		for i := start; i <= end; i++ {
			expandedString := re.ReplaceAllString(input, strconv.Itoa(i))
			expandedStrings = append(expandedStrings, expandedString)
		}

		return expandedStrings, nil
	}

	return nil, nil
}

// extracted from franz-go
func Murmur2(b []byte) uint32 {
	const (
		seed uint32 = 0x9747b28c
		m    uint32 = 0x5bd1e995
		r           = 24
	)
	h := seed ^ uint32(len(b))
	for len(b) >= 4 {
		k := uint32(b[3])<<24 + uint32(b[2])<<16 + uint32(b[1])<<8 + uint32(b[0])
		b = b[4:]
		k *= m
		k ^= k >> r
		k *= m

		h *= m
		h ^= k
	}
	switch len(b) {
	case 3:
		h ^= uint32(b[2]) << 16
		fallthrough
	case 2:
		h ^= uint32(b[1]) << 8
		fallthrough
	case 1:
		h ^= uint32(b[0])
		h *= m
	}

	h ^= h >> 13
	h *= m
	h ^= h >> 15
	return h
}

func instanceInternalIDToShard(internalID []byte, numShards int) uint32 {
	return Murmur2(internalID) % uint32(numShards)
}

// TruncatedSHA256 truncates the SHA256 hash to 32 characters
func TruncatedSHA256(id string) ([]byte, error) {
	h := sha256.New()
	_, err := h.Write([]byte(id))
	if err != nil {
		return nil, fmt.Errorf("error in h.Write: %w", err)
	}

	// Use a truncated hash
	return h.Sum(nil)[:32], nil
}

func ptr[T any](s T) *T {
	return &s
}
