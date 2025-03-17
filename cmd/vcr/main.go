package main

import (
	"fmt"
	"os"
	"path"
	"vcrserver/client"
	"vcrserver/server"
)

func main() {
	if len(os.Args) != 3 && len(os.Args) != 4 {
		fmt.Printf("usage: %s <record url|serve>  <sqlite.db>\n", os.Args[0])
		os.Exit(1)
	}

	switch path.Base(os.Args[1]) {
	case "record":
		client.Run(os.Args[2], os.Args[3])
	case "serve":
		server.Run(os.Args[2])
	default:
		fmt.Printf("usage: %s <client|server> <url> <sqlite.db>\n", os.Args[0])
		os.Exit(1)
	}
}
