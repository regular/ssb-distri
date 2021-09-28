package main

import (
  "fmt"
  "log"
  "io/ioutil"
  "net/url"
  "net/http"
	"google.golang.org/protobuf/encoding/prototext"
	"github.com/distr1/distri/pb"
)

type MetaFetcher struct {
  base *url.URL
  client *http.Client
}

func (mf *MetaFetcher) fetchMeta(pkgname string) *pb.Meta {
  metaUrlString := fmt.Sprintf("./%v.meta.textproto", pkgname)
  metaUrl, err := url.Parse(metaUrlString)
	if err != nil { log.Fatal(err) }

  absUrl := mf.base.ResolveReference(metaUrl)

	resp, err := mf.client.Get(absUrl.String())
	if err != nil { log.Fatal(err) }
	defer resp.Body.Close()

  bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil { log.Fatal(err) }
  
  var meta pb.Meta
  err = prototext.Unmarshal(bytes, &meta)
  if err != nil { log.Fatal(err) }
  return &meta
}
