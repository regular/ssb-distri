package main

import (
	//"fmt"
  "io"
	"log"
  "golang.org/x/net/html"
  "golang.org/x/net/html/atom"
)

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
