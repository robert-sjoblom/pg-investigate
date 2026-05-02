package main

import (
	"fmt"

	"github.com/alecthomas/kong"
)

var cli struct {
	Investigation string `short:"i" required:"" help:"Investigation name"`
	Time          string `short:"t" required:"" help:"Incident time"`
	User          string `short:"u" required:"" help:"SSH username"`
	Output        string `short:"o" default:"./investigation" help:"Output dir"`
	Host          string `name:"host" required:"" help:"SSH target"`
	Vm            string `name:"vm" required:"" help:"Harvester VM name"`
	Namespace     string `name:"ns" required:"" help:"Kubernetes namespace"`
}

func main() {
	ctx := kong.Parse(&cli)
	fmt.Printf("%+v\n", ctx)
}
