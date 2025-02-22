package main

import (
	cryptoRand "crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"os/signal"
	"slices"
)

const helpText = `randoo - randomize the order of exec args

USAGE:
  randoo [options] [--] command [args...]

DESCRIPTION:
  By default, randoo will call command after shuffling args.
  How args are provided and shuffled may be controlled using options.

OPTIONS:
`

type CLI struct {
	Input  io.Reader
	Output io.Writer
	ErrOut io.Writer

	scanLines     bool
	shuffleAfter  string
	shuffleBefore string

	rand    *rand.Rand
	flagSet *flag.FlagSet
	command string
	args    []string
	prep    func() error
}

func main() {
	os.Exit((&CLI{
		Input:  os.Stdin,
		Output: os.Stdout,
		ErrOut: os.Stderr,
	}).Main(os.Args[1:]))
}

func (x *CLI) Main(args []string) int {
	if err := x.Init(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}

		_, _ = fmt.Fprintf(x.ErrOut, "ERROR: %s\n", err)
		return 2
	}

	if err := x.Run(); err != nil {
		// pass through the exit code
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr != nil {
			if status, _ := exitErr.Sys().(interface{ ExitStatus() int }); status != nil {
				if v := status.ExitStatus(); v != 0 {
					return v
				}
			}
		}

		_, _ = fmt.Fprintf(x.ErrOut, "ERROR: %s\n", err)
		return 1
	}

	return 0
}

func (x *CLI) Init(args []string) error {
	x.flagSet = flag.NewFlagSet(`randoo`, flag.ContinueOnError)
	x.flagSet.Usage = x.usage
	x.flagSet.SetOutput(x.Output)
	x.flagSet.BoolVar(&x.scanLines, `l`, false, `Read input from stdin, one arg per line. Appended after any trailing args, which are _not_ shuffled.`)
	x.flagSet.StringVar(&x.shuffleAfter, `s`, ``, `Shuffle args after the specified arg (start delimiter). If not found, an error will occur. Not passed.`)
	x.flagSet.StringVar(&x.shuffleBefore, `e`, ``, `Shuffle args before the specified arg (end delimiter). If not found, an error will occur. Not passed.`)

	if err := x.flagSet.Parse(args); err != nil {
		return err
	}

	// require a command, extract remaining args
	x.args = x.flagSet.Args()
	if len(x.args) == 0 {
		return fmt.Errorf("no command specified")
	}
	x.command = x.args[0]
	x.args = x.args[1:]

	switch {
	case x.scanLines:
		x.prep = x.prepScanLines
	default:
		x.prep = x.prepDefault
	}

	x.rand = rand.New(&randSource{})

	return nil
}

func (x *CLI) Run() error {
	if err := x.prep(); err != nil {
		return err
	}

	sigs := make(chan os.Signal, 512)
	defer close(sigs)
	signal.Notify(sigs)
	defer signal.Stop(sigs)

	cmd := exec.Command(x.command, x.args...)
	cmd.Stdin = x.Input
	cmd.Stdout = x.Output
	cmd.Stderr = x.ErrOut

	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		for sig := range sigs {
			_ = cmd.Process.Signal(sig)
		}
	}()

	return cmd.Wait()
}

func (x *CLI) usage() {
	if x.Output != nil {
		_, _ = x.Output.Write([]byte(helpText))
		x.flagSet.PrintDefaults()
	}
}

func (x *CLI) shuffle(args []string) ([]string, error) {
	args = slices.Clone(args)

	if x.shuffleAfter == `` && x.shuffleBefore == `` {
		x.rand.Shuffle(len(args), func(i, j int) {
			args[i], args[j] = args[j], args[i]
		})
		return args, nil
	}

	{
		var token string

		after := x.shuffleAfter != ``
		{
			before := x.shuffleBefore != ``
			if !after || !before {
				// only handles the one delimiter case
				if after {
					token = x.shuffleAfter
				} else {
					token = x.shuffleBefore
				}
			}
		}

		if token != `` {
			index := -1
			for i, arg := range args {
				if arg == token {
					index = i
					break
				}
			}
			if index == -1 {
				return nil, fmt.Errorf("shuffle delimiter not found: %q", token)
			}

			// remove the delimiter
			copy(args[index:], args[index+1:])
			args = args[:len(args)-1]

			var shuffle []string

			if after {
				shuffle = args[index:]
			} else {
				shuffle = args[:index]
			}

			x.rand.Shuffle(len(shuffle), func(i, j int) {
				shuffle[i], shuffle[j] = shuffle[j], shuffle[i]
			})

			return args, nil
		}
	}

	var ok bool

	for i := 0; i < len(args); i++ {
		if args[i] == x.shuffleAfter {
			i++
			start := i

			{
				var ok bool
				for ; i < len(args); i++ {
					if args[i] == x.shuffleBefore {
						ok = true
						break
					}
				}
				if !ok {
					return nil, fmt.Errorf("shuffle end delimiter not found: %q", x.shuffleBefore)
				}
			}

			l := i - start
			x.rand.Shuffle(i-start, func(i, j int) {
				args[start+i], args[start+j] = args[start+j], args[start+i]
			})

			// remove the delimiters
			copy(args[start-1:], args[start:i])
			copy(args[start-1+l:], args[i+1:])
			args = args[:len(args)-2]
			i -= 2

			ok = true
		}
	}

	if !ok {
		return nil, fmt.Errorf("shuffle start delimiter not found: %q", x.shuffleAfter)
	}

	return args, nil
}

func (x *CLI) prepDefault() error {
	args, err := x.shuffle(x.args)
	if err != nil {
		return err
	}
	x.args = args
	return nil
}

func (x *CLI) prepScanLines() error {
	var lines []string

	if x.Input != nil {
		for {
			var line string
			_, err := fmt.Fscanln(x.Input, &line)

			if err != nil && !errors.Is(err, io.EOF) {
				return err
			}

			if err == nil || line != `` {
				lines = append(lines, line)
			}

			if err != nil {
				break
			}
		}
	}

	lines, err := x.shuffle(lines)
	if err != nil {
		return err
	}

	x.args = append(x.args, lines...)

	return nil
}

type randSource [8]byte

func (x *randSource) Uint64() uint64 {
	if _, err := cryptoRand.Read(x[:]); err != nil {
		panic(err)
	}
	return binary.LittleEndian.Uint64(x[:])
}
