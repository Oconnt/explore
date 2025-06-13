package cmd

import "github.com/urfave/cli"

const (
	usage = `explore is a process exploration tool that provides interaction with go processes, 
             controlling them by manipulating process memory`
)

func NewExp() *cli.App {
	app := cli.NewApp()
	app.Name = "exp"
	app.Usage = usage
	app.Commands = []cli.Command{
		read,
		write,
		list,
		attach,
		conn,
	}

	return app
}
