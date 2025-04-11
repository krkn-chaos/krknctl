package cache

import "sync"

type Cache interface {
	SetString(key string, value string)
	GetString(key string) string
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
	c.memoryCache[key] = []byte(value)
}

func (c *cache) GetString(key string) string {
	return string(c.memoryCache[key])
}

func (c *cache) Set(key string, value []byte) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.memoryCache[key] = value
}

func (c *cache) Get(key string) []byte {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.memoryCache[key]
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
