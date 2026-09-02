package cache

import (
	"sync"
	"time"
)

type item struct {
	value      any
	expiration int64
}

type Cache struct {
	items sync.Map
}

func New() *Cache {
	return &Cache{}
}

func (c *Cache) Set(key string, value any, duration time.Duration) {
	c.items.Store(key, item{
		value:      value,
		expiration: time.Now().Add(duration).UnixNano(),
	})
}

func (c *Cache) Get(key string) (any, bool) {
	val, ok := c.items.Load(key)
	if !ok {
		return nil, false
	}

	it := val.(item)
	if time.Now().UnixNano() > it.expiration {
		c.items.Delete(key)
		return nil, false
	}

	return it.value, true
}
