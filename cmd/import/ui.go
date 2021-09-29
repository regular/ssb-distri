package main

import (
  "fmt"
  "time"
  "strings"
  "github.com/atomicgo/cursor"
)

type StatusUpdate struct {
  slot  int
  start time.Time
  name string
  status string
}

func StatusMonitor() chan StatusUpdate {
  ch := make(chan StatusUpdate)
  slots := make([]*StatusUpdate, 8)
  tick := time.Tick(time.Second / 8)
  go func() {
    for {
      select {
      case u := <- ch:
        //fmt.Println("update")
        slots[u.slot] = &u      
        
      case <- tick:
        //fmt.Println("tick")
        cursor.Bottom()
        for _, u := range slots {
          cursor.Move(0, 1)
          cursor.ClearLine()
          cursor.StartOfLine()
          if u != nil {
            if u.name != "" {
              fmt.Printf("%v) %v: %v", u.slot, formatName(u.name), u.status)
            } else {
              fmt.Printf("%v) %v", u.slot, u.status)
            }
            fmt.Printf(" %v", time.Since(u.start).Round(time.Second))
          }
        }
      }
    }
  }()

  return ch
}

func formatName(n string) string {
  const wanted = 40
  if len(n)>wanted {
    n = n[:wanted+1]
  } else {
    n += strings.Repeat(" ", wanted - len(n))
  }
  return n
}
