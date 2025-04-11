package cache

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

func TestCache_Set(t *testing.T) {
	testString := "test"
	cache := NewCache()
	cache.Set("test", []byte(testString))
	cacheGet := cache.Get("test")
	assert.NotNil(t, cacheGet)
	assert.Equal(t, []byte(testString), cacheGet)
	cacheGetStr := cache.GetString("test")
	assert.Equal(t, testString, *cacheGetStr)

	strings := []string{"test-a", "test-b", "test-c", "test-d", "test-e"}

	var wg sync.WaitGroup

	for i, str := range strings {
		wg.Add(1)
		go func(s string, index int) {
			defer wg.Done()
			cache.SetString(s, fmt.Sprintf("test-%d", index))
		}(str, i)
	}

	wg.Wait()

	for _, str := range strings {
		value := cache.GetString(str)
		assert.NotNil(t, value)
	}

	cache.Delete("test-a")
	assert.Nil(t, cache.GetString("test-a"))
	assert.NotNil(t, cache.GetString("test-b"))
	cache.Invalidate()
	assert.Nil(t, cache.GetString("test-b"))

}
