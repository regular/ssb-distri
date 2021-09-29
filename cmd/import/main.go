package main

import (
	"fmt"
	"log"
  "flag"
	"net/http"
	"net/url"
	"os"
  "strings"
	"google.golang.org/protobuf/encoding/prototext"

	"github.com/distr1/distri/pb"
)


var (
  localRepoDir string
  startURL string = "https://repo.distr1.org/distri/supersilverhaze/pkg/"
  workers int = 4
  downloaders int = 4

  client http.Client
  cache *Cache
  fetcher MetaFetcher
)

func main() {
  flag.StringVar(&startURL, "remote", "https://repo.distr1.org/distri/supersilverhaze/pkg/", "URL of remote distri package repository")
  flag.IntVar(&workers, "workers", 4, "Number of parallel crawlers gathering package meta data")
  flag.IntVar(&downloaders, "downloaders", 4, "Number of parallel downloads")

  flag.Parse()

  if len(flag.Args()) < 1 {
    fmt.Println("Missing argument: path to local package repo direcotry")
    os.Exit(1)
  }
  if len(flag.Args()) > 1 {
    fmt.Println("Too many arguments")
    os.Exit(1)
  }

  localRepoDir = flag.Args()[0]
  
  stat, err := os.Stat(localRepoDir)
  if err != nil {
    fmt.Println(err)
    os.Exit(1)
  }
  if !stat.IsDir() {
    fmt.Printf("%v is not a directory!\n", localRepoDir)
    os.Exit(1)
  }

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

  monitor := StatusMonitor()
  ch_links := make(chan string, 20)
  ch_needed := make(chan string)
  ch_done_workers := make(chan bool)
  ch_done_downloaders := make(chan bool)

  for i:=0; i<downloaders; i++ {
    go download(i, base, &client, monitor, ch_needed, ch_done_downloaders)
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
  close(monitor)

}

func download(index int, base *url.URL, client *http.Client, monitor chan StatusUpdate, needed chan string, done chan bool) {
  downloader := Downloader{
    base,
    client,
  }
  for pkgname := range needed {
    //fmt.Printf("downloading %v\n", pkgname)
    cache.Lock()
    meta := cache.Get(pkgname)
    cache.Unlock()
    metaString := prototext.Format(meta)
    //fmt.Println(metaString)
    _ = metaString
    progress := make(chan StatusUpdate)
    go func() {
      for p := range progress {
        p.slot = index
        monitor <- p
      }
      
    }()
    downloader.downloadFile(pkgname, localRepoDir, progress)

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
    promise <- fetcher.fetchMeta(dep)

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

  meta := fetcher.fetchMeta(pkgname)
  promise <- meta
  
  version := *meta.Version
  //fmt.Println(version)
  versionSuffix := fmt.Sprintf("-%v", version)
  if strings.HasSuffix(pkgname, versionSuffix) {
    //pkgname = strings.TrimSuffix(pkgname, versionSuffix)
  } else {
    latest := fmt.Sprintf("%v-%v", pkgname, version)
    //fmt.Printf("  latest: %s\n", latest)
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
  //fmt.Println("Worker done.")
  done <- true
}

