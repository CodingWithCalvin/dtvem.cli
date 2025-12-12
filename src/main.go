package main

import (
	"github.com/dtvem/dtvem/src/cmd"

	// Import runtime providers to register them
	_ "github.com/dtvem/dtvem/src/runtimes/node"
	_ "github.com/dtvem/dtvem/src/runtimes/python"
	_ "github.com/dtvem/dtvem/src/runtimes/ruby"
)

func main() {
	cmd.Execute()
}
