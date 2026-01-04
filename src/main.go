package main

import (
	"github.com/CodingWithCalvin/dtvem.cli/src/cmd"

	// Import runtime providers to register them
	_ "github.com/CodingWithCalvin/dtvem.cli/src/runtimes/node"
	_ "github.com/CodingWithCalvin/dtvem.cli/src/runtimes/python"
	_ "github.com/CodingWithCalvin/dtvem.cli/src/runtimes/ruby"

	// Import migration providers to register them
	// Node.js migration providers
	_ "github.com/CodingWithCalvin/dtvem.cli/src/migrations/node/fnm"
	_ "github.com/CodingWithCalvin/dtvem.cli/src/migrations/node/nvm"
	_ "github.com/CodingWithCalvin/dtvem.cli/src/migrations/node/system"

	// Python migration providers
	_ "github.com/CodingWithCalvin/dtvem.cli/src/migrations/python/pyenv"
	_ "github.com/CodingWithCalvin/dtvem.cli/src/migrations/python/system"

	// Ruby migration providers
	_ "github.com/CodingWithCalvin/dtvem.cli/src/migrations/ruby/chruby"
	_ "github.com/CodingWithCalvin/dtvem.cli/src/migrations/ruby/rbenv"
	_ "github.com/CodingWithCalvin/dtvem.cli/src/migrations/ruby/rvm"
	_ "github.com/CodingWithCalvin/dtvem.cli/src/migrations/ruby/system"
	_ "github.com/CodingWithCalvin/dtvem.cli/src/migrations/ruby/uru"
)

func main() {
	cmd.Execute()
}
