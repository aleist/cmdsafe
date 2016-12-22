package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var (
	dbPath  = "data.db" // The path to the DB file.
	subCmd  command     // The active sub-command.
	cmdName string      // The handle for the cmd to modify or execute.
)

// command is the type of a valid sub-command.
type command string

// Valid sub-commands.
const (
	cmdDelete command = "delete"
	cmdRun    command = "run"
	cmdSave   command = "save"
)

func init() {
	// Define the usage message.
	flag.Usage = func() {
		path := strings.Split(os.Args[0], string(os.PathSeparator))
		fmt.Fprintf(os.Stderr, "Usage: %s [global flags, ...] command [cmd flags, ...]\n\n",
			path[len(path)-1])

		fmt.Fprintln(os.Stderr, "Global flags:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nThe commands are:")
		fmt.Fprintln(os.Stderr, "  delete\tdelete a saved cmd")
		fmt.Fprintln(os.Stderr, "  run   \trun a saved cmd")
		fmt.Fprintln(os.Stderr, "  save  \tsave a new or update an existing cmd")
	}

	// Parse general arguments.
	flag.StringVar(&dbPath, "db", dbPath, "The database `path`")
	flag.Parse()

	// Parse sub-command name and arguments.
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: missing command name\n")
		flag.Usage()
		os.Exit(2)
	}
	switch command(args[0]) {
	case cmdDelete:
		subCmd = cmdDelete
		initCmdDelete(args[1:])
	case cmdRun:
		subCmd = cmdRun
		initCmdRun(args[1:])
	case cmdSave:
		subCmd = cmdSave
		initCmdSave(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		flag.Usage()
		os.Exit(2)
	}
}

func initCmdDelete(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: delete [flags] <cmd name>\n")
		os.Exit(2)
	}

	cmdName = args[len(args)-1]
}

// initCmdRun parses arguments specific to command 'run'.
func initCmdRun(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: run [flags] <cmd name>\n")
		os.Exit(2)
	}

	cmdName = args[len(args)-1]
}

func initCmdSave(args []string) {
	flags := flag.NewFlagSet("save", flag.ExitOnError)

	flags.StringVar(&cmdName, "name", "", "The name used to refer to the saved cmd")

	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Usage: save [flags]\n")
		flags.PrintDefaults()
		os.Exit(2)
	}
}

func main() {
}
