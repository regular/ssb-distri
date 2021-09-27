package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	//"os"
  "strings"
	"google.golang.org/protobuf/encoding/prototext"

	"github.com/distr1/distri/pb"
)

const startURL = "https://repo.distr1.org/distri/supersilverhaze/pkg/"
const workers = 4
const downloaders = 4

var (
  client http.Client
  cache *Cache
  fetcher MetaFetcher
)

//href pattern
// ./accountsservice-amd64.meta.textproto
// (contains version field)
// ./accountsservice-amd64-0.6.55-12.meta.textproto
// ./accountsservice-amd64-0.6.55-12.squashfs.zst

  /*
  fileName = "list.txt"
	// Create blank file
	file, err := os.Create(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
  */

func main() {
  client = http.Client{
    CheckRedirect: func(r *http.Request, via []*http.Request) error {
      r.URL.Opaque = r.URL.Path
      return nil
    },
  }
  
  base, err := url.Parse(startURL)
	if err != nil { log.Fatal(err) }
  fetcher = MetaFetcher{
    base,
    &client,
  }

  cache = NewCache()

  ch_links := make(chan string, 20)
  ch_needed := make(chan string)
  ch_done_workers := make(chan bool)
  ch_done_downloaders := make(chan bool)

  for i:=0; i<downloaders; i++ {
    go download(ch_needed, ch_done_downloaders)
  }

  for i:=0; i<workers; i++ {
    go processLinks(ch_links, ch_done_workers, ch_needed)
  }
  links(startURL, ch_links)

  for i:=0; i<workers; i++ {
    <- ch_done_workers
  }
  close(ch_needed)
  for i:=0; i<downloaders; i++ {
    <- ch_done_downloaders
  }

}

func download(needed chan string, done chan bool) {
  for pkgname := range needed {
    fmt.Printf("downloading %v\n", pkgname)
    cache.Lock()
    meta := cache.Get(pkgname)
    cache.Unlock()
    metaString := prototext.Format(meta)
    fmt.Println(metaString)
  }
  done <- true
}


func walkDeps(meta *pb.Meta, needed chan string) {
  //fmt.Printf("  SourcePkg: %v\n", *meta.SourcePkg)
  for _, dep := range meta.RuntimeDep {
    //fmt.Printf("  - dep: %v\n", dep)

    cache.Lock()
    if cache.Has(dep) {
      //fmt.Println("    (cached)")
      cache.Unlock()
      continue
    } 
    promise := cache.AddPromise(dep)
    cache.Unlock()
    promise <- fetcher.fetch(dep)

    needed <- dep
  }
}

func getPkgnameFromUrl(metaUrlString string) string {
  metaUrl, err := url.Parse(metaUrlString) 
	if err != nil { log.Fatal(err) }
  sliced := strings.Split(metaUrl.Path, "/")
  pkgname := sliced[len(sliced)-1]
  pkgname = strings.TrimSuffix(pkgname, ".meta.textproto")
  return pkgname
}

func visit(metaUrlString string, needed chan string) {
  pkgname := getPkgnameFromUrl(metaUrlString)
  //fmt.Printf("  pkgname: %v\n", pkgname)
  
  cache.Lock()
  if cache.Has(pkgname) {
    cache.Unlock()
    //fmt.Println("  (cached)")
    return
  }
  // channel serves as a promise for meta
  promise := cache.AddPromise(pkgname)
  cache.Unlock()

  meta := fetcher.fetch(pkgname)
  promise <- meta
  
  version := *meta.Version
  //fmt.Println(version)
  versionSuffix := fmt.Sprintf("-%v", version)
  if strings.HasSuffix(pkgname, versionSuffix) {
    //pkgname = strings.TrimSuffix(pkgname, versionSuffix)
  } else {
    latest := fmt.Sprintf("%v-%v", pkgname, version)
    fmt.Printf("  latest: %s\n", latest)
    cache.Lock()
    promise := cache.AddPromise(latest)
    cache.Unlock()
    promise <- meta
    needed <- latest
    walkDeps(meta, needed)
  }
}

func processLinks(urls chan string, done chan bool, needed chan string) {
  for s := range urls {
    if strings.HasSuffix(s, ".meta.textproto") {
      //fmt.Printf("- %q\n", s)
      visit(s, needed)
    }
  }
  fmt.Println("Worker done.")
  done <- true
}

