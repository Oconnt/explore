package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/urfave/cli"
	"os"
	"strings"
)

const (
	ExactArgs = iota
	minArgs
	maxArgs
)

func CheckArgs(context *cli.Context, expected, checkType int, fn func(args cli.Args) error) error {
	var err error
	cmdName := context.Command.Name
	switch checkType {
	case ExactArgs:
		if context.NArg() != expected {
			err = fmt.Errorf("%s: %q requires exactly %d argument(s)", os.Args[0], cmdName, expected)
		}
	case minArgs:
		if context.NArg() < expected {
			err = fmt.Errorf("%s: %q requires a minimum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	case maxArgs:
		if context.NArg() > expected {
			err = fmt.Errorf("%s: %q requires a maximum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	}

	if err != nil {
		fmt.Printf("Incorrect Usage.\n\n")
		_ = cli.ShowCommandHelp(context, cmdName)
		return err
	}

	return fn(context.Args())
}

func PrefixIn(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}

	return false
}

func SuffixIn(s string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}

	return false
}

func MD5(s string) string {
	hasher := md5.New()
	hasher.Write([]byte(s))
	hashSlice := hasher.Sum(nil)
	return hex.EncodeToString(hashSlice)
}
