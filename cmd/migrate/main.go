package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/GLEZH/gofermart-graduate-project/internal/adapter/postgres"
	"github.com/GLEZH/gofermart-graduate-project/internal/config"
)

func main() {
	uri := os.Getenv("DATABASE_URI")
	if uri == "" {
		uri = config.DatabaseURIFromEnv()
	}
	flags := flag.NewFlagSet("migrate", flag.ExitOnError)
	flags.StringVar(&uri, "d", uri, "database connection string")
	_ = flags.Parse(os.Args[1:])
	action := "up"
	if flags.NArg() > 0 {
		action = flags.Arg(0)
	}
	database, err := postgres.NewDatabase(context.Background(), uri)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	if action == "down" {
		err = database.Down()
	} else {
		err = database.Migrate()
	}
	if err != nil {
		log.Fatal(err)
	}
}
