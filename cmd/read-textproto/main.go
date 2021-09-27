package main

import (
  "os"
  "fmt"
  "io/ioutil"
  "encoding/json"

	"github.com/distr1/distri/pb"
	"google.golang.org/protobuf/encoding/prototext"
)

func main() {
    bytes, err := ioutil.ReadAll(os.Stdin)
    if err != nil {
      os.Exit(1)
    }

    var buildProto pb.Build
    err = prototext.Unmarshal(bytes, &buildProto)
    if err != nil {
      os.Exit(1)
    }
    enc, err := json.MarshalIndent(&buildProto, "", "  ")
    if err := prototext.Unmarshal(bytes, &buildProto); err != nil {
      os.Exit(1)
    }
    fmt.Printf(string(enc))

}
