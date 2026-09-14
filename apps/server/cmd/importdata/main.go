package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"vts-tokenmaxers/apps/server/internal/store"
)

func main() {
	databasePath := flag.String("database", "./data/silent-outposts.db", "SQLite database path")
	sourcePath := flag.String("source", "./data/source/sunken-garden-and-cs-building.zip", "dataset ZIP or unpacked directory")
	flag.Parse()

	dataStore, err := store.OpenDatabase(context.Background(), *databasePath)
	if err != nil {
		fatal(err)
	}
	defer dataStore.Close()
	report, err := dataStore.ImportSource(context.Background(), *sourcePath)
	if err != nil {
		fatal(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
