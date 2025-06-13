package cmd

import (
	"explore/pkg/prowler"
	"explore/utils"
	"fmt"
	"github.com/urfave/cli"
	"strconv"
)

var list = cli.Command{
	Name:  "ls",
	Usage: "display all global variable or constant names in the process",
	Flags: []cli.Flag{
		cli.IntFlag{
			Name:  "type, t",
			Value: int(prowler.Vac),
			Usage: "this selection specifies whether constants, variables, or all are listed",
		},
		cli.StringSliceFlag{
			Name:  "prefixes, p",
			Usage: "prefix filtering",
		},
		cli.StringSliceFlag{
			Name:  "suffixes, s",
			Usage: "suffix filtering",
		},
	},
	Action: func(context *cli.Context) error {
		if err := utils.CheckArgs(context, 1, utils.ExactArgs, listArgsCheck); err != nil {
			return err
		}

		pidStr := context.Args().First()
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			return err
		}

		return exec(List, pid, context)
	},
}

func listArgsCheck(args cli.Args) error {
	pid := args.First()

	if !utils.CheckPid(pid) {
		return fmt.Errorf("pid %s does not exist", pid)
	}

	return nil
}
