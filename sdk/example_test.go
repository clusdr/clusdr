package clusdr_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/clusdr/clusdr/sdk"
)

// These examples need a local daemon (clusdr init && clusdr start --bootstrap).
// They are compiled with the module tests but not executed (no Output).

func Example() {
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	members, err := c.Members(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(members), "members")

	if err := c.Publish(ctx, "deployment", []byte(`{"sha":"abc"}`)); err != nil {
		log.Fatal(err)
	}
}

func ExampleLocal() {
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err) // daemon down, TLS, or Health not ready within 10s
	}
	defer c.Close()
}

func ExampleDial() {
	// Second daemon on this host, or a test listener — not a remote peer.
	c, err := clusdr.Dial("127.0.0.1:8947", clusdr.WithDataDir("./data-b"))
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
}

func ExampleCluster_Watch() {
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := c.Watch(ctx, clusdr.WithTopics("deployment"))
	if err != nil {
		log.Fatal(err)
	}
	for ev := range ch {
		fmt.Println(ev.Type, ev.Source)
	}
}

func ExampleCluster_Lock() {
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lk, err := c.Lock(ctx, "scheduler", 15*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(lk.Name, lk.Token)
	if err := c.Unlock(ctx, "scheduler"); err != nil {
		log.Fatal(err)
	}
}

func ExampleCluster_TryLock() {
	c, err := clusdr.Local()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lk, ok, err := c.TryLock(ctx, "scheduler", 15*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	if !ok {
		fmt.Println("held by", lk.Holder)
		return
	}
	fmt.Println("acquired", lk.Token)
	if err := c.Unlock(ctx, "scheduler"); err != nil {
		log.Fatal(err)
	}
}
