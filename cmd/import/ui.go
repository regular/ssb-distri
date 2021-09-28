package main

import (
  "fmt"
  "github.com/atomicgo/cursor"
)

type StatusUpdate struct {
  slot  int
  status string
}

func StatusMonitor() chan StatusUpdate {
  ch := make(chan StatusUpdate)

  go func() {
    for u := range ch {
      cursor.Bottom()
      cursor.Move(0, u.slot +1)
      cursor.ClearLine()
      fmt.Printf("%v: %v", u.slot, u.status)
    }
  }()

  return ch
}
