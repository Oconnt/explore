package cmd

import (
	"explore/utils"
	"fmt"
	"github.com/urfave/cli"
	"time"
)

var conn = cli.Command{
	Name:  "conn",
	Usage: "connect an explore window",
	Action: func(context *cli.Context) error {
		if err := utils.CheckArgs(context, 1, utils.ExactArgs, connArgsCheck); err != nil {
			return err
		}

		return exec(Conn, 0, context)
	},
}

func connArgsCheck(args cli.Args) error {
	addr := args.First()
	if utils.Telnet(addr, 5*time.Second) {
		return nil
	}

	return fmt.Errorf("invalid connection address: %s", addr)
}
