// Hold an exclusive cluster lock the way a scheduler would.
//
//	clusdr init && clusdr start --bootstrap
//	go run ./examples/scheduler
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/durguto/clusdr/sdk"
)

func main() {
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	lk, err := c.Lock(ctx, "scheduler", 15*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("held %s holder=%s token=%d until %s\n", lk.Name, lk.Holder, lk.Token, lk.Deadline().UTC().Format(time.RFC3339))

	// Store lk.Token with any write that must be fenced.
	time.Sleep(2 * time.Second)

	if err := c.Unlock(ctx, "scheduler"); err != nil {
		log.Fatal(err)
	}
	fmt.Println("released scheduler")
}
