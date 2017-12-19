package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/korylprince/competition-scorer/api"
	"github.com/korylprince/competition-scorer/db"
)

var addr = flag.String("addr", "0.0.0.0", "address to listen on")
var port = flag.Int("port", 8080, "port to listen on")
var path = flag.String("path", "competition.db", "path to competition database")
var reset = flag.Bool("reset", false, "used to reset username and password")
var user = flag.String("user", "", "set username to given value (use with -reset)")
var password = flag.String("pass", "", "set password to given value (use with -reset)")

func printUsage() {
	fmt.Println("Usage:", os.Args[0], "[options]")
	flag.PrintDefaults()
}

func resetPassword(path, username, password string) error {
	d, err := db.New(path)
	if err != nil {
		return err
	}

	return d.UpdateCredentials(username, password)
}

func main() {
	flag.Usage = printUsage
	flag.Parse()

	if *reset {
		if *user == "" {
			fmt.Println("Error: -user must be set if using -reset")
			printUsage()
			return
		}
		if *password == "" {
			fmt.Println("Error: -pass must be set if using -reset")
			printUsage()
			return
		}

		err := resetPassword(*path, *user, *password)
		if err != nil {
			fmt.Println("Error: Could not reset credentials:", err)
			return
		}

		fmt.Println("Credentials reset successfully")
		return
	}

	if *user != "" || *password != "" {
		fmt.Println("Error: -reset must be used if using -user or -pass")
		printUsage()
		return
	}

	d, err := db.New(*path)
	if err != nil {
		fmt.Println("Error: Could not open database", path, ":", err)
		return
	}

	r := api.NewRouter(d, api.NewMemorySessionStore(time.Hour*8), api.NewSubscribeService())

	fmt.Println("Listening on", fmt.Sprintf("%s:%d", *addr, *port))
	err = http.ListenAndServe(fmt.Sprintf("%s:%d", *addr, *port), r)
	if err != nil {
		fmt.Println("Error serving on", fmt.Sprintf("%s:%d", *addr, *port), ":", err)
	}

}
