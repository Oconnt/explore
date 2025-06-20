package cmd

import (
	"explore/pkg/logflags"
	"explore/utils"
	"fmt"
	"github.com/urfave/cli"
	"strconv"
)

var attach = cli.Command{
	Name:  "attach",
	Usage: "attach to a process",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "logFlag, f",
			Usage: "enable debug logging",
		},
		cli.StringFlag{
			Name:  "logStr, s",
			Usage: "specify the type of logger",
			Value: "http",
		},
		cli.StringFlag{
			Name:  "logDesc, d",
			Usage: "specify the log file path",
			Value: logflags.DefaultLogDesc,
		},
	},
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
