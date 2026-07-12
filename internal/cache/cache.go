package cache

import (
	"net/http"

	gomem "github.com/bradfitz/gomemcache/memcache"
	httpcache "github.com/gregjones/httpcache"
	cachemem "github.com/gregjones/httpcache/memcache"
)

type Cache struct {
	client *http.Client
	mc     *gomem.Client
	tp     *httpcache.Transport
	cache  *cachemem.Cache
	addr   string
}

func NewDefault() *Cache {
	return New("127.0.0.1:11211")
}

func New(addr string) *Cache {
	mc := gomem.New(addr)

	cache := cachemem.NewWithClient(mc)

	tp := httpcache.NewTransport(cache)

	client := &http.Client{Transport: tp}

	return &Cache{client: client, mc: mc, tp: tp, cache: cache, addr: addr}
}

func (c *Cache) Client() *http.Client {
	return c.client
}

func (c *Cache) Flush() error {
	return c.mc.FlushAll()
}
