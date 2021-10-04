package main

import (
	"fmt"
	"log"
  "flag"
	"net/http"
	"net/url"
	"os"
  "sync"
  "strings"
	"github.com/distr1/distri/pb"
)


var (
  localRepoDir string
  startURL string = "https://repo.distr1.org/distri/supersilverhaze/pkg/"
  workers int = 4
  downloaders int = 4
  showProgress bool = true

  client http.Client
  cache *Cache
  isNeeded sync.Map
  fetcher MetaFetcher
  repo Repo 
)

func main() {
  flag.StringVar(&startURL, "remote", "https://repo.distr1.org/distri/supersilverhaze/pkg/", "URL of remote distri package repository")
  flag.IntVar(&workers, "workers", 4, "Number of parallel crawlers gathering package meta data")
  flag.IntVar(&downloaders, "downloaders", 4, "Number of parallel downloads")
  flag.BoolVar(&showProgress, "progress", true, "Whether to show download progress")

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

  repo = FSRepo{localRepoDir}

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
  ch_quit := make(chan bool)
  var monitor chan StatusUpdate

  if showProgress {
    monitor = StatusMonitor(ch_quit)
    defer close(monitor)
  }

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
  log.Println("Quit")
  if showProgress {
    ch_quit <- true
  }
}

func download(index int, base *url.URL, client *http.Client,  monitor chan StatusUpdate, needed chan string, done chan bool) {
  downloader := Downloader{
    base,
    client,
  }
  for pkgname := range needed {
    progress := make(chan StatusUpdate)
    go func() {
      for p := range progress {
        p.slot = index
        if showProgress {
          monitor <- p
        }
      }
    }()
    digest, err := downloader.downloadFile(pkgname, repo, progress)
    if err != nil { log.Fatal(err) }
    if digest != nil { // else it existed already
      meta := GetMeta(pkgname)
      err =  repo.AddPackage(pkgname, digest, meta)
      if err != nil { log.Fatal(err) }
    }
  }
  done <- true
}


func walkDeps(pkgname string, meta *pb.Meta, needed chan string) {
  //fmt.Printf("  SourcePkg: %v\n", *meta.SourcePkg)
  for _, dep := range meta.RuntimeDep {
    //fmt.Printf("  - dep: %v\n", dep)

    _, ok := isNeeded.LoadOrStore(pkgname, true)
    if ok {
      log.Printf("    - dep %v is already needed\n", dep)
      continue
    } 

    log.Printf("    - %v is needed as dependency of %v\n", dep, pkgname)
    
    GetMeta(pkgname)
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

func GetMeta(pkgname string) *pb.Meta {
  var meta *pb.Meta
  cache.Lock()
  if cache.Has(pkgname) {
    meta = cache.Get(pkgname)
    cache.Unlock()
  } else {
    // channel serves as a promise for meta
    promise := cache.AddPromise(pkgname)
    cache.Unlock()
    meta = fetcher.fetchMeta(pkgname)
    promise <- meta
  }
  return meta
}

func visit(metaUrlString string, needed chan string) {
  pkgname := getPkgnameFromUrl(metaUrlString)
  //fmt.Printf("  pkgname: %v\n", pkgname)
  _, ok := isNeeded.Load(pkgname)
  if ok { return }

  meta := GetMeta(pkgname)
  version := *meta.Version

  versionSuffix := fmt.Sprintf("-%v", version)
  if !strings.HasSuffix(pkgname, versionSuffix) {
    latest := fmt.Sprintf("%v-%v", pkgname, version)

    err := repo.MarkAsCurrentVersion(latest, meta)
    if err != nil { log.Fatal(err) }

    _, ok = isNeeded.LoadOrStore(latest, true)
    if ok {
      log.Printf("latest version of %v is already needed.\n", pkgname)
      return 
    }

    cache.Lock()
    promise := cache.AddPromise(latest)
    cache.Unlock()
    promise <- meta

    log.Printf("We need: %v, because %v links to it, marking it as latest\n", latest, pkgname)
    needed <- latest
    walkDeps(latest, meta, needed)
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

