package cmd

import (
	"explore/utils"
	"fmt"
	"github.com/urfave/cli"
	"strconv"
)

var attach = cli.Command{
	Name:  "attach",
	Usage: "attach to a process",
	Action: func(context *cli.Context) error {
		if err := utils.CheckArgs(context, 1, utils.ExactArgs, attachArgsCheck); err != nil {
			return err
		}

		pid, err := strconv.Atoi(context.Args().First())
		if err != nil {
			return err
		}
		return exec(Attach, pid, context)
	},
}

func attachArgsCheck(args cli.Args) error {
	pid := args.First()
	if !utils.CheckPid(pid) {
		return fmt.Errorf("pid %s does not exist", pid)
	}

	return nil
}
