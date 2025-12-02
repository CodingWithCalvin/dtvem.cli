package main

import (
	"github.com/dtvem/dtvem/src/cmd"

	// Import runtime providers to register them
	_ "github.com/dtvem/dtvem/src/runtimes/node"
	_ "github.com/dtvem/dtvem/src/runtimes/python"
)

func main() {
	cmd.Execute()
}
