package main

import (
	cache "charles/goCache"
	"fmt"
	"time"
)

func main() {
	// Create a cache with a default expiration time of 5 minutes, and which
	// purges expired items every 30 seconds
	c := cache.New(10*time.Minute, 30*time.Second)

	// Set the value of the key "foo" to "bar", with the default expiration time
	c.Set("foo", "bar", cache.DefaultExpiration)

	// Get the string associated with the key "foo" from the cache
	//foo, found := c.Get("foo")
	if foo, found := c.Get("foo"); found {
		fmt.Println(foo)
	}
	// Set the value of the key "num" to 10, with the default expiration time.And add 1 to it.
	c.Set("num", 10, cache.DefaultExpiration)
	err1 := c.Increment("num", 1)
	if err1 != nil {
		fmt.Println(err1)
	}
	if num, found := c.Get("num"); found {
		fmt.Println(num)
	}
	//Replace the value of item "foo"
	err := c.Replace("foo", "change", cache.DefaultExpiration)
	if err != nil {
		fmt.Println(err)
	}

	if foo, found := c.Get("foo"); found {
		fmt.Println(foo)
	}
	//Get the number of the item in the cache
	c.Set("test", "hehe", cache.DefaultExpiration)
	num := c.ItemCount()
	fmt.Println(num)
	//Delete the item in the cache
	c.Delete("foo")
	if _, found := c.Get("foo"); !found {
		fmt.Println("delete")
	}

}
