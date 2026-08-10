// cleanup_sweep registers the callback that removes stale learning records.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	cleanupcron "example.com/edtech-cleanup-sweep"
)

func main() {
	taskURL := flag.String("task", os.Getenv("CLEANUP_TASK_URL"), "HTTPS cleanup callback URL")
	cronExpr := flag.String("schedule", "15 2 * * *", "five-field UTC cron expression")
	flag.Parse()
	if *taskURL == "" {
		log.Fatal("set CLEANUP_TASK_URL or pass -task")
	}

	client, err := cleanupcron.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	jobID, err := client.CronCreate(context.Background(), *cronExpr, *taskURL)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("cleanup sweep registered:", jobID)
}
