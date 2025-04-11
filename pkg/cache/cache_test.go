package cache

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCache_Set(t *testing.T) {
	testString := "test"
	cache := NewCache()
	cache.Set("test", []byte(testString))
	cache.Get("test")
	assert.Equal(t, []byte(testString), cache.Get("test"))
	assert.Equal(t, testString, cache.GetString(testString))

	strings := []string{"test-a", "test-b", "test-c", "test-d", "test-e"}

	for string := range strings {
		go
	}


}
