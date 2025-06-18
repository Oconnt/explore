package terminal

import (
	"errors"
	"explore/service"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

var (
	argumentsErr = "invalid number of arguments, expected %d, actual %d"
)

type cmdPrefix int

const (
	noPrefix = cmdPrefix(0)
	onPrefix = cmdPrefix(1 << iota)
	deferredPrefix
	revPrefix
)

type cmdFn func(term *Term, args string) error

type command struct {
	aliases         []string
	allowedPrefixes cmdPrefix
	fn              cmdFn
	help            string
}

func (c command) match(cmdstr string) bool {
	for _, v := range c.aliases {
		if v == cmdstr {
			return true
		}
	}
	return false
}

type Commands struct {
	cmds   []command
	client service.Client
}

func NewCommands(client service.Client) *Commands {
	c := &Commands{
		client: client,
	}

	c.cmds = []command{
		{
			aliases: []string{"help", "h"},
			fn:      c.help,
			help: `Prints the help message.

	help [command]

Type "help" followed by the name of a command for more information about it.`},
		{
			aliases: []string{"get", "g"},
			fn:      get,
			help:    "retrieve variable, constant, or function information of the target process through remote calling.",
		},
		{
			aliases: []string{"set", "s"},
			fn:      set,
			help:    "modify the corresponding variable information of the process.",
		},
		{
			aliases: []string{"list", "ls"},
			fn:      list,
			help:    "list lists information such as variables, constants, functions, etc. by specifying prefixes, types, etc. Detailed information can be obtained through the get command.",
		},
		{
			aliases: []string{"exit", "quit", "q"},
			fn:      exit,
			help:    "exit the exp",
		},
	}
	return c
}

// Find will look up the command function for the given command input.
// If it cannot find the command it will default to noCmdAvailable().
// If the command is an empty string it will replay the last command.
func (c *Commands) Find(cmdstr string, prefix cmdPrefix) command {
	// If <enter> use last command, if there was one.
	if cmdstr == "" {
		return command{aliases: []string{"nullcmd"}, fn: nullCommand}
	}

	for _, v := range c.cmds {
		if v.match(cmdstr) {
			if prefix != noPrefix && v.allowedPrefixes&prefix == 0 {
				continue
			}
			return v
		}
	}

	return command{aliases: []string{"nocmd"}, fn: noCmdAvailable}
}

func (c *Commands) Call(cmdStr string, t *Term) error {
	cmd, argStr, _ := strings.Cut(cmdStr, " ")

	return c.Find(cmd, noPrefix).fn(t, argStr)
}

func (c *Commands) help(t *Term, args string) error {
	fmt.Fprintln(t.stdout, "The following commands are available:")
	w := new(tabwriter.Writer)
	w.Init(t.stdout, 0, 8, 0, '-', 0)
	for _, cmd := range c.cmds {
		h := cmd.help
		if idx := strings.Index(h, "\n"); idx >= 0 {
			h = h[:idx]
		}
		if len(cmd.aliases) > 1 {
			fmt.Fprintf(w, "    %s (alias: %s) \t %s\n", cmd.aliases[0], strings.Join(cmd.aliases[1:], " | "), h)
		} else {
			fmt.Fprintf(w, "    %s \t %s\n", cmd.aliases[0], h)
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Fprintln(t.stdout)
	// fmt.Fprintln(t.stdout, "Type help followed by a command for full documentation.")
	return nil
}

func get(t *Term, args string) error {
	v, err := t.client.SendExpr(service.Get, args)
	if err != nil {
		t.RedirectTo(os.Stderr)
		fmt.Fprintln(t.stdout, err.Error())
		return err
	}

	_, err = fmt.Fprintln(t.stdout, v)
	return err
}

func set(t *Term, args string) error {
	v, err := t.client.SendExpr(service.Set, args)
	if err != nil {
		t.RedirectTo(os.Stderr)
		fmt.Fprintln(t.stdout, err.Error())
		return err
	}

	_, err = fmt.Fprintln(t.stdout, v)
	return err
}

func list(t *Term, args string) error {
	vs, err := t.client.SendExpr(service.List, args)
	if err != nil {
		t.RedirectTo(os.Stderr)
		fmt.Fprintln(t.stdout, err.Error())
		return err
	}

	_, err = fmt.Fprintln(t.stdout, vs)
	return err
}

type ExitRequestError struct{}

func (ere ExitRequestError) Error() string {
	return ""
}

func exit(t *Term, args string) error {
	return ExitRequestError{}
}

var errNoCmd = errors.New("command not available")

func noCmdAvailable(t *Term, args string) error {
	return errNoCmd
}

func nullCommand(t *Term, args string) error {
	return nil
}
