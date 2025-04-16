package cache

import "sync"

type Cache interface {
	SetString(key string, value string)
	GetString(key string) *string
	Set(key string, value []byte)
	Get(key string) []byte
	Delete(key string)
	Invalidate()
}

var (
	instance *cache
	once     sync.Once
)

func NewCache() Cache {
	once.Do(func() {
		instance = &cache{
			memoryCache: make(map[string][]byte),
		}
	})
	return instance
}

type cache struct {
	memoryCache map[string][]byte
	lock        sync.Mutex
}

func (c *cache) SetString(key string, value string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.memoryCache[key] = []byte(value)
}

func (c *cache) GetString(key string) *string {
	c.lock.Lock()
	defer c.lock.Unlock()
	if value, ok := c.memoryCache[key]; ok {
		retVal := string(value)
		return &retVal
	}
	return nil
}

func (c *cache) Set(key string, value []byte) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.memoryCache[key] = value
}

func (c *cache) Get(key string) []byte {
	c.lock.Lock()
	defer c.lock.Unlock()
	ret := make([]byte, 0)
	if value, ok := c.memoryCache[key]; ok {
		return value
	}
	return ret
}

func (c *cache) Delete(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.memoryCache, key)
}

func (c *cache) Invalidate() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.memoryCache = make(map[string][]byte)
}
