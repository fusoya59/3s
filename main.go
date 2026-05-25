package main

import (
	"os"

	"github.com/fusoya59/3s/cmd"
)

func main() {
	os.Exit(cmd.Run(os.Args))
}
