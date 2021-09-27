package main

import (
  "sync"
  "log"
	"github.com/distr1/distri/pb"
)

type Cache struct {
  sync.Mutex
  promises map[string] chan *pb.Meta
  entries map[string] *pb.Meta
}
func NewCache() *Cache {
  cache := Cache{
    promises: make(map[string] chan *pb.Meta),
    entries: make(map[string] *pb.Meta),
  }
  return &cache
}

// caller MUST own lock
// and later write to the returned channel
// to fullfill promise
// fullfilling the promise does NOT require locking
func (c *Cache) AddPromise(pkgname string) chan *pb.Meta {
  ch := make(chan *pb.Meta, 1)
  c.promises[pkgname] = ch
  return ch
}

// caller must own lock
func (c *Cache) Get(pkgname string) *pb.Meta {
  meta, ok := c.entries[pkgname]
  if ok { return meta }

  ch, ok := c.promises[pkgname]
  if !ok { log.Fatal("not cached:", pkgname) }
  entry := <- ch
  delete (c.promises, pkgname)
  c.entries[pkgname] = entry
  return entry
}

// caller MUST own lock
func (c *Cache) Has(pkgname string) bool {
  _, ok := c.entries[pkgname]
  if ok { return true }

  _, ok = c.promises[pkgname]
  if ok { return true }
  return false
}

