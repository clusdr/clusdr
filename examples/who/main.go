// Who is in the cluster, and who leads.
//
//	clusdr init && clusdr start --bootstrap
//	go run ./examples/who
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/durguto/clusdr/sdk"
)

func main() {
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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tADDRESS\tSTATUS\tROLE\tLEADER")
	for _, m := range members {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%v\n", m.ID, m.Address, m.Status, m.Role, m.Leader)
	}
	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}

	leader, err := c.Leader(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("leader %s at %s\n", leader.ID, leader.Address)
}
