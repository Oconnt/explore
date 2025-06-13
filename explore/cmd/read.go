package cmd

import (
	"explore/utils"
	"fmt"
	"github.com/urfave/cli"
	"strconv"
	"strings"
)

var read = cli.Command{
	Name:  "get",
	Usage: "read to processes",
	Action: func(context *cli.Context) error {
		if err := utils.CheckArgs(context, 2, utils.ExactArgs, readArgsCheck); err != nil {
			return err
		}

		pidStr := context.Args().First()
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			return err
		}

		return exec(Get, pid, context)
	},
}

type readArgs struct {
	name string
}

func rArgs(args cli.Args) *readArgs {
	return &readArgs{
		name: args.Get(1),
	}
}

func readArgsCheck(args cli.Args) error {
	pid := args.First()
	name := args.Get(1)

	if !utils.CheckPid(pid) {
		return fmt.Errorf("pid %s does not exist", pid)
	}

	if !strings.Contains(name, ".") {
		return fmt.Errorf("variable name must contain '.'")
	}

	return nil
}
