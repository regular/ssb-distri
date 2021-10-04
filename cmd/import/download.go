package main

import (
  "time"
  "fmt"
  "log"
  "io"
  "net/http"
	"net/url"
  "github.com/klauspost/compress/zstd"
)

type Downloader struct {
  base *url.URL
  client *http.Client
}

func (d *Downloader) urlFromPkg(pkgname string) string {
  relUrlString := fmt.Sprintf("./%v.squashfs.zst", pkgname)
  relUrl, err := url.Parse(relUrlString)
	if err != nil { log.Fatal(err) }

  absUrl := d.base.ResolveReference(relUrl)
  return absUrl.String()
}


func (d *Downloader) downloadFile(pkgname string, target Repo, progress chan StatusUpdate) (digest interface{}, err error) {
  fileUrl := d.urlFromPkg(pkgname)
  defer close(progress)

  if target.HasPackage(pkgname) {
    log.Printf("%v already exists.\n", pkgname)
    progress <- StatusUpdate{
      start: time.Now(),
      name: pkgname,
      status: "already exists",
    }
    return
  }

  decompress, err := zstd.NewReader(nil)
  if err != nil { log.Fatal(err) }
  defer decompress.Close()

  blob, err := target.NewBlobWriter(pkgname)
	if err != nil { log.Fatal(err) }
	defer blob.Close()

  log.Printf("Downloading %v to %v\n", fileUrl, blob.String())

	resp, err := d.client.Get(fileUrl)
	if err != nil { log.Fatal(err) }
	defer resp.Body.Close()

  //buffer := make([]byte, 1<<15) // 32k buffer
  buffer := make([]byte, 1<<14) // 16k
  decompress.Reset(resp.Body)
  reader := io.TeeReader(decompress, blob)
  total := 0
  start := time.Now()
  for {
    n, err := reader.Read(buffer)
    total += n
    progress <- StatusUpdate{
      start: start,
      name: pkgname,
      status: fmt.Sprintf("%10.d bytes", total),
    }
    if err != nil { 
      if err != io.EOF { log.Fatal(err) }
      break
    }
  }
  //fmt.Println("Done.")
  return blob.Digest()
}
