package main

import (
	"github.com/dtvem/dtvem/src/cmd"

	// Import runtime providers to register them
	_ "github.com/dtvem/dtvem/src/runtimes/node"
	_ "github.com/dtvem/dtvem/src/runtimes/python"
	_ "github.com/dtvem/dtvem/src/runtimes/ruby"

	// Import migration providers to register them
	// Node.js migration providers
	_ "github.com/dtvem/dtvem/src/migrations/node/fnm"
	_ "github.com/dtvem/dtvem/src/migrations/node/nvm"
	_ "github.com/dtvem/dtvem/src/migrations/node/system"

	// Python migration providers
	_ "github.com/dtvem/dtvem/src/migrations/python/pyenv"
	_ "github.com/dtvem/dtvem/src/migrations/python/system"

	// Ruby migration providers
	_ "github.com/dtvem/dtvem/src/migrations/ruby/chruby"
	_ "github.com/dtvem/dtvem/src/migrations/ruby/rbenv"
	_ "github.com/dtvem/dtvem/src/migrations/ruby/rvm"
	_ "github.com/dtvem/dtvem/src/migrations/ruby/system"
)

func main() {
	cmd.Execute()
}
