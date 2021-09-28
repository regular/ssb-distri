package main

import (
  "net/http"
	"net/url"
  "path/filepath"
  "fmt"
  "log"
  "os"
  "io"
  "io/ioutil"
  "github.com/klauspost/compress/zstd"
)

type Downloader struct {
  base *url.URL
  client *http.Client
}

func (d *Downloader) filenameFromPkg(pkgname string) string {
  return fmt.Sprintf("%v.squashfs", pkgname)
}

func (d *Downloader) urlFromPkg(pkgname string) string {
  relUrlString := fmt.Sprintf("./%v.zst", d.filenameFromPkg(pkgname))
  relUrl, err := url.Parse(relUrlString)
	if err != nil { log.Fatal(err) }

  absUrl := d.base.ResolveReference(relUrl)
  return absUrl.String()
}

func (d *Downloader) downloadFile(pkgname, targetDir string, progress chan string) {
  fileUrl := d.urlFromPkg(pkgname)
  fileName := d.filenameFromPkg(pkgname)
  targetPath := filepath.Join(targetDir, fileName) 
  defer close(progress)

  if _, err := os.Stat(targetPath); err == nil {
    //fmt.Printf("%v already exists.\n", fileName)
    return
  }

  decompress, err := zstd.NewReader(nil)
  if err != nil { log.Fatal(err) }
  defer decompress.Close()

	file, err := ioutil.TempFile(targetDir, "download-*.part")
	if err != nil { log.Fatal(err) }
	defer file.Close()
  //tmpName := file.Name()
  //defer os.Remove(tmpName) // will fail if download completes

  //fmt.Printf("Downloading %v to %v\n", fileUrl, file.Name())

	resp, err := d.client.Get(fileUrl)
	if err != nil { log.Fatal(err) }
	defer resp.Body.Close()

  buffer := make([]byte, 1<<14) // 16k buffer
  decompress.Reset(resp.Body)
  reader := io.TeeReader(decompress, file)
  total := 0
  for {
    n, err := reader.Read(buffer)
    total += n
    progress <- fmt.Sprintf("%v: %v bytes", fileName, total)
    if err != nil { 
      if err != io.EOF { log.Fatal(err) }
      break
    }
  }
  //fmt.Println("Done.")
  err = os.Rename(file.Name(), targetPath)
  if err != nil {
    log.Fatal(err)
  }

}
