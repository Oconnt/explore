package terminal

import (
	"errors"
	"explore/service"
	"fmt"
	"github.com/derekparker/trie"
	"github.com/go-delve/liner"
	"io"
	"os"
	"os/signal"
	"os/user"
	"path"
	"strings"
	"syscall"
)

const (
	prompt                             = "(exp) "
	expDir                             = ".explore"
	historyFile                 string = ".exp_history"
	terminalHighlightEscapeCode string = "\033[%2dm"
	terminalResetEscapeCode     string = "\033[0m"
)

type Term struct {
	client      service.Client
	prompt      string
	line        *liner.State
	cmds        *Commands
	historyFile *os.File
	stdout      *transcriptWriter
}

func New(client service.Client) *Term {
	t := &Term{
		client: client,
		line:   liner.NewLiner(),
		prompt: prompt,
		stdout: &transcriptWriter{pw: &pagingWriter{w: os.Stdout}},
		cmds:   NewCommands(client),
	}

	return t
}

//func (t *Term) sigintGuard(ch <-chan os.Signal, multiClient bool) {
//	for range ch {
//		t.longCommandCancel()
//		t.starlarkEnv.Cancel()
//		state, err := t.client.GetStateNonBlocking()
//		if err == nil && state.Recording {
//			fmt.Fprintf(t.stdout, "received SIGINT, stopping recording (will not forward signal)\n")
//			err := t.client.StopRecording()
//			if err != nil {
//				fmt.Fprintf(os.Stderr, "%v\n", err)
//			}
//			continue
//		}
//		if err == nil && state.CoreDumping {
//			fmt.Fprintf(t.stdout, "received SIGINT, stopping dump\n")
//			err := t.client.CoreDumpCancel()
//			if err != nil {
//				fmt.Fprintf(os.Stderr, "%v\n", err)
//			}
//			continue
//		}
//		if multiClient {
//			answer, err := t.line.Prompt("Would you like to [p]ause the target (returning to Delve's prompt) or [q]uit this client (leaving the target running) [p/q]? ")
//			if err != nil {
//				fmt.Fprintf(os.Stderr, "%v", err)
//				continue
//			}
//			answer = strings.TrimSpace(answer)
//			switch answer {
//			case "p":
//				_, err := t.client.Halt()
//				if err != nil {
//					fmt.Fprintf(os.Stderr, "%v", err)
//				}
//			case "q":
//				t.quittingMutex.Lock()
//				t.quitting = true
//				t.quittingMutex.Unlock()
//				err := t.client.Disconnect(false)
//				if err != nil {
//					fmt.Fprintf(os.Stderr, "%v", err)
//				} else {
//					t.Close()
//				}
//			default:
//				fmt.Fprintln(t.stdout, "only p or q allowed")
//			}
//		} else {
//			fmt.Fprintf(t.stdout, "received SIGINT, stopping process (will not forward signal)\n")
//			_, err := t.client.Halt()
//			if err != nil {
//				fmt.Fprintf(t.stdout, "%v", err)
//			}
//		}
//	}
//}

func (t *Term) sigintGuard(ch <-chan os.Signal) {
	for range ch {
		fmt.Fprintf(t.stdout, "received SIGINT, stopping process (will not forward signal)\n")
	}
}

func (t *Term) Run() error {
	defer t.Close()

	var (
		err error
	)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go t.sigintGuard(ch)

	cmds := trie.New()
	for _, cmd := range t.cmds.cmds {
		for _, alias := range cmd.aliases {
			cmds.Add(alias, nil)
		}
	}

	t.line.SetCompleter(func(line string) (c []string) {
		// cmd := t.cmds.Find(strings.Split(line, " ")[0], onPrefix)
		c = cmds.PrefixSearch(line)
		return
	})

	userHomeDir := getUserHomeDir()
	fullHistory := path.Join(userHomeDir, expDir, historyFile)

	t.historyFile, err = os.OpenFile(fullHistory, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(parentDir(fullHistory), 0755); err != nil {
				return fmt.Errorf("create parent dir failed: %v", err)
			}

			t.historyFile, err = os.OpenFile(fullHistory, os.O_CREATE|os.O_RDWR, 0600)
		} else {
			fmt.Printf("Unable to open history file: %v. History will not be saved for this session.\n", err)
			return err
		}
	}

	if _, err = t.line.ReadHistory(t.historyFile); err != nil {
		fmt.Printf("Unable to read history file %s: %v\n", fullHistory, err)
		return err
	}

	fmt.Println("Type 'help' for list of commands.")

	for {
		cmd, err := t.promptForInput()
		if err != nil {
			if err == io.EOF {
				fmt.Fprintln(t.stdout, "exit")
				return t.handleExit()
			}
			return errors.New("Prompt for input failed.\n")
		}
		t.stdout.Echo(t.prompt + cmd + "\n")

		if strings.TrimSpace(cmd) == "" {
			continue
		}

		if err = t.cmds.Call(cmd, t); err != nil {
			if _, ok := err.(ExitRequestError); ok {
				return t.handleExit()
			}

			fmt.Fprintf(os.Stderr, "Command failed: %s\n", err)
		}

		t.stdout.Flush()
		t.stdout.pw.Reset()
	}
}

func (t *Term) Close() {
	t.line.Close()
	if err := t.stdout.CloseTranscript(); err != nil {
		fmt.Fprintf(os.Stderr, "error closing transcript file: %v\n", err)
	}
}

func getUserHomeDir() string {
	userHomeDir := "."
	usr, err := user.Current()
	if err == nil {
		userHomeDir = usr.HomeDir
	}
	return userHomeDir
}

func (t *Term) promptForInput() (string, error) {
	//if t.stdout.colorEscapes != nil && t.conf.PromptColor != "" {
	//	fmt.Fprint(os.Stdout, t.conf.PromptColor)
	//	defer fmt.Fprint(os.Stdout, terminalResetEscapeCode)
	//}
	l, err := t.line.Prompt(t.prompt)
	if err != nil {
		return "", err
	}

	l = strings.TrimSuffix(l, "\n")
	if l != "" {
		t.line.AppendHistory(l)
	}

	return l, nil
}

func (t *Term) handleExit() error {
	if t.historyFile != nil {
		if _, err := t.line.WriteHistory(t.historyFile); err != nil {
			fmt.Println("readline history error:", err)
			return err
		}
		if err := t.historyFile.Close(); err != nil {
			fmt.Printf("error closing history file: %s\n", err)
			return err
		}
	}

	return nil
}

// RedirectTo redirects the output of this terminal to the specified writer.
func (t *Term) RedirectTo(w io.Writer) {
	t.stdout.pw.w = w
}

func parentDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == os.PathSeparator {
			return path[:i]
		}
	}
	return ""
}
