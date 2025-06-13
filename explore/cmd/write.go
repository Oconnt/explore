package cmd

import (
	"explore/utils"
	"github.com/urfave/cli"
	"strconv"
)

var write = cli.Command{
	Name:  "set",
	Usage: "writing a process variable is unsafe, as unexpected situations may occur if multiple command lines are concurrent.",
	Action: func(context *cli.Context) error {
		if err := utils.CheckArgs(context, 3, utils.ExactArgs, writeArgsCheck); err != nil {
			return err
		}

		pid, err := strconv.Atoi(context.Args().First())
		if err != nil {
			return err
		}

		return exec(Set, pid, context)
	},
}

type writeArgs struct {
	name  string
	value string
}

func wArgs(args cli.Args) *writeArgs {
	return &writeArgs{
		name:  args.Get(1),
		value: args.Get(2),
	}
}

func writeArgsCheck(args cli.Args) error {
	return readArgsCheck(args)
}
