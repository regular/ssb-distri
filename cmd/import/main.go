package main

import (
	"fmt"
	"io"
  "io/ioutil"
	"log"
  "sync"
	"net/http"
	"net/url"
	//"os"
  "strings"
  "golang.org/x/net/html"
  "golang.org/x/net/html/atom"
	"google.golang.org/protobuf/encoding/prototext"

	"github.com/distr1/distri/pb"
)

const startURL = "https://repo.distr1.org/distri/supersilverhaze/pkg/"
const workers = 4
const downloaders = 4
var client http.Client

type Cache struct {
  sync.Mutex
  entries map[string] chan *pb.Meta
}
var cache Cache

func (cache *Cache) Add(pkgname string) chan *pb.Meta {
  ch := make(chan *pb.Meta, 1)
  cache.entries[pkgname] = ch
  return ch
}
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
  cache = Cache{}
  cache.entries = make(map[string] chan *pb.Meta)
  
  client = http.Client{
    CheckRedirect: func(r *http.Request, via []*http.Request) error {
      r.URL.Opaque = r.URL.Path
      return nil
    },
  }

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
    if ch, ok := cache.entries[pkgname]; !ok {
      log.Fatal("No meta data for %v", pkgname)
    } else {
      cache.Unlock()
      meta := <- ch
      metaString := prototext.Format(meta)
      fmt.Println(metaString)
    }
  }
  done <- true
}

func makeAbsMetaUrl(pkgname string) string {
  base, err := url.Parse(startURL)
	if err != nil { log.Fatal(err) }

  metaUrlString := fmt.Sprintf("./%v.meta.textproto", pkgname)
  metaUrl, err := url.Parse(metaUrlString)
	if err != nil { log.Fatal(err) }

  absUrl := base.ResolveReference(metaUrl)
  return absUrl.String()
}

func getMeta(absMetaUrl string) *pb.Meta {
	resp, err := client.Get(absMetaUrl)
	if err != nil { log.Fatal(err) }
	defer resp.Body.Close()

  bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil { log.Fatal(err) }
  
  var meta pb.Meta
  err = prototext.Unmarshal(bytes, &meta)
  if err != nil { log.Fatal(err) }
  return &meta
}

func walkDeps(meta *pb.Meta, needed chan string) {
  //fmt.Printf("  SourcePkg: %v\n", *meta.SourcePkg)
  for _, dep := range meta.RuntimeDep {
    //fmt.Printf("  - dep: %v\n", dep)

    cache.Lock()
    if _, ok := cache.entries[dep]; ok {
      //fmt.Println("    (cached)")
      cache.Unlock()
      continue
    } 
    metaChan := cache.Add(dep)
    cache.Unlock()
    metaChan <- getMeta(makeAbsMetaUrl(dep))
    needed <- dep
  }
}

func visit(metaUrlString string, needed chan string) {
  metaUrl, err := url.Parse(metaUrlString) 
	if err != nil { log.Fatal(err) }
  sliced := strings.Split(metaUrl.Path, "/")
  pkgname := sliced[len(sliced)-1]
  pkgname = strings.TrimSuffix(pkgname, ".meta.textproto")
  //fmt.Printf("  pkgname: %v\n", pkgname)
  
  cache.Lock()
  if _, ok := cache.entries[pkgname]; ok {
    cache.Unlock()
    //fmt.Println("  already in cache")
    return
  }
  // channel serves as a promise for meta
  metaChan := cache.Add(pkgname)
  cache.Unlock()

  base, err := url.Parse(startURL)
	if err != nil { log.Fatal(err) }

  absUrl := base.ResolveReference(metaUrl)
  //fmt.Println(absUrl)
  meta := getMeta(absUrl.String())
  metaChan <- meta
  
  version := *meta.Version
  //fmt.Println(version)
  versionSuffix := fmt.Sprintf("-%v", version)
  if strings.HasSuffix(pkgname, versionSuffix) {
    //pkgname = strings.TrimSuffix(pkgname, versionSuffix)
  } else {
    latest := fmt.Sprintf("%v-%v", pkgname, version)
    fmt.Printf("  latest: %s\n", latest)
    cache.Lock()
    cache.Add(latest) <- meta
    cache.Unlock()
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

func links(url string, c chan string) {
  defer close(c)

	resp, err := client.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	//size, err := io.Copy(file, resp.Body)
  z := html.NewTokenizer(resp.Body)
  for {
    next := z.Next()
    if next == html.ErrorToken {
      err := z.Err()
      if err != io.EOF {
        log.Print(err)
      }
      break
    }
    switch token := z.Token(); token.Type {
      case html.StartTagToken: {
        if token.DataAtom == atom.A {
          for _, a := range token.Attr {
            if a.Key == "href" {
              //fmt.Print(a.Val)
              c <- a.Val
            }
          }
        }
      }
    }
  }
}
