package goCache

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

const (

	// For use with functions that take an expiration time.
	NoExpiration time.Duration = -1

	DefaultExpiration time.Duration = 0
)

//
type Item struct {
	Object     interface{}
	Expiration int64
}

//return true if the item expirated
func (item Item) Expired() bool {
	if item.Expiration == 0 {
		return false
	}

	return time.Now().UnixNano() > item.Expiration
}

//use for outer struct
type Cache struct {
	*cache
}

//
type cache struct {
	defaultExpiration time.Duration
	items             map[string]Item
	mu                sync.RWMutex
	onEvicted         func(string, interface{}) //callback
	janitor           *janitor
}

/////////cache function

// Add an item to the cache, replacing any existing item.
func (c *cache) Set(k string, x interface{}, d time.Duration) {
	var e int64
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}

	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}
	c.mu.Lock()
	c.items[k] = Item{
		Object:     x,
		Expiration: e,
	}
	c.mu.Unlock()

}

func (c *cache) set(k string, x interface{}, d time.Duration) {
	var e int64
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}

	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}
	c.items[k] = Item{
		Object:     x,
		Expiration: e,
	}
}

func (c *cache) get(k string) (interface{}, bool) {
	item, found := c.items[k]
	if !found {
		return nil, false
	}
	//check item expired
	if item.Expiration > 0 && item.Expiration < time.Now().UnixNano() {
		return nil, false
	}
	return item.Object, true
}

// Add an item to the cache only if an item doesn't already exist for the given
// key, or if the existing item has expired. Returns an error otherwise.
func (c *cache) Add(k string, x interface{}, d time.Duration) error {
	c.mu.Lock()
	//check exit
	_, found := c.get(k)
	if found {
		err := fmt.Errorf("Item:%s has already exit", k)
		return err
	}
	//set
	c.set(k, x, d)
	c.mu.Unlock()
	return nil
}

// Set a new value for the cache key only if it already exists, and the existing
// item hasn't expired. Returns an error otherwise.
func (c *cache) Replace(k string, x interface{}, d time.Duration) error {
	c.mu.Lock()
	//check exit
	_, found := c.get(k)
	if !found {
		c.mu.Unlock()
		err := fmt.Errorf("Item:%s dosen't exit", k)
		return err
	}
	c.set(k, x, d)
	c.mu.Unlock()
	return nil
}

//Increment an item of number (int, TODO other type).Returns an error if the
// item's value is not an integer, if it was not found, or if it is not
// possible to increment it by n. To retrieve the incremented value, use one
// of the specialized methods, e.g. IncrementInt64.
func (c *cache) Increment(k string, n int64) error {
	c.mu.Lock()
	v, found := c.items[k]
	if !found || v.Expired() {
		c.mu.Unlock()
		return fmt.Errorf("Item not found or expired")
	}
	switch v.Object.(type) {
	case int:
		v.Object = v.Object.(int) + int(n)
	default:
		c.mu.Unlock()
		return fmt.Errorf("not support value tyepe")
	}
	c.items[k] = v
	c.mu.Unlock()
	return nil
}

// Get an item from the cache. Returns the item or nil, and a bool indicating
// whether the key was found.
func (c *cache) Get(k string) (interface{}, bool) {
	c.mu.Lock()
	item, found := c.items[k]
	if !found {
		c.mu.Unlock()
		return nil, false
	}
	//check item expired
	if item.Expiration < time.Now().UnixNano() {
		c.mu.Unlock()
		return nil, false
	}
	c.mu.Unlock()
	return item.Object, true

}

func (c *cache) delete(k string) (interface{}, bool) {
	//
	if c.onEvicted != nil {
		if v, found := c.items[k]; found {
			delete(c.items, k)
			return v.Object, true
		}
	}
	delete(c.items, k)
	return nil, false
}

// Delete an item from the cache. Does nothing if the key is not in the cache.
func (c *cache) Delete(k string) {
	c.mu.Lock()
	v, evicted := c.delete(k)
	c.mu.Unlock()
	if evicted {
		c.onEvicted(k, v)
	}

}

type kv struct {
	key   string
	value interface{}
}

// Delete all expired items from the cache.
func (c *cache) DeleteExpired() {
	var evictedItems []kv
	timeNow := time.Now().UnixNano()
	c.mu.Lock()
	for k, v := range c.items {
		if v.Expiration > 0 && v.Expiration < timeNow {
			v, evicted := c.delete(k)
			if evicted {
				evictedItems = append(evictedItems, kv{k, v})
			}
		}
	}
	c.mu.Unlock()
	for _, v := range evictedItems {
		c.onEvicted(v.key, v.value)
	}
}

//Return the item in the cache
func (c *cache) Item() map[string]Item {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.items
}

// Return the number of items in the cache. Equivalent to len(c.Items()).
func (c *cache) ItemCount() int {
	c.mu.Lock()
	n := len(c.items)
	c.mu.Unlock()
	return n

}

// Delete all items from the cache.
func (c *cache) Flush() {
	c.mu.Lock()
	c.items = map[string]Item{}
	c.mu.Unlock()

}

//////////// janitor function

type janitor struct {
	Interval time.Duration
	stop     chan bool
}

func (j *janitor) Run(c *cache) {
	j.stop = make(chan bool)
	ticker := time.NewTicker(j.Interval)
	for {
		select {
		case <-ticker.C:
			//delete expired
			c.DeleteExpired()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

//////////

func stopJanitor(c *Cache) {
	c.janitor.stop <- true
}

func runJanitor(c *cache, ci time.Duration) {
	j := &janitor{
		Interval: ci,
	}

	c.janitor = j

	go j.Run(c)

}

func newCache(de time.Duration, m map[string]Item) *cache {
	if de == 0 {
		de = -1
	}
	c := &cache{
		defaultExpiration: de,
		items:             m,
	}
	return c
}

func newCacheWithJanitor(defaultExpiration time.Duration, cleanupInterval time.Duration, items map[string]Item) *Cache {
	c := newCache(defaultExpiration, items)
	//init Cache
	C := &Cache{c}
	if cleanupInterval > 0 {
		runJanitor(c, cleanupInterval)
		runtime.SetFinalizer(C, stopJanitor)
	}
	return C
}

// Return a new cache with a given default expiration duration and cleanup interval.
func New(defaultExpiration, cleanupInterval time.Duration) *Cache {
	items := make(map[string]Item)
	return newCacheWithJanitor(defaultExpiration, cleanupInterval, items)
}
