package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var (
	progName string  // The name of this executable.
	subCmd   command // The active sub-command.

	dbPath    = "data.db" // The path to the DB file.
	cmdHandle string      // The handle for the cmd to modify or execute.
)

// command is the type of a valid sub-command.
type command string

// Valid sub-commands.
const (
	deleteCommand command = "delete"
	listCommand   command = "list"
	runCommand    command = "run"
	saveCommand   command = "save"
)

func init() {
	// Extract the program name from the path.
	path := strings.Split(os.Args[0], string(os.PathSeparator))
	progName = path[len(path)-1]

	// Define the usage message.
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [global flags, ...] command [flags, ...]\n\n", progName)
		fmt.Fprintln(os.Stderr, "Global flags:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nThe commands are:")
		fmt.Fprintln(os.Stderr, "  delete\tdelete a saved cmd")
		fmt.Fprintln(os.Stderr, "  list  \tlist all saved cmds")
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
	case deleteCommand:
		subCmd = deleteCommand
		initCmdDelete(args[1:])
	case listCommand:
		subCmd = listCommand
	case runCommand:
		subCmd = runCommand
		initCmdRun(args[1:])
	case saveCommand:
		subCmd = saveCommand
		initCmdSave(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		flag.Usage()
		os.Exit(2)
	}
}

// initCmdDelete parses arguments specific to sub-command 'delete'.
func initCmdDelete(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: delete <cmd name>\n")
		os.Exit(2)
	}

	cmdHandle = args[len(args)-1]
}

// initCmdRun parses arguments specific to sub-command 'run'.
func initCmdRun(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: run <cmd name>\n")
		os.Exit(2)
	}

	cmdHandle = args[len(args)-1]
}

// initCmdSave parses arguments specific to sub-command 'save'.
func initCmdSave(args []string) {
	flags := flag.NewFlagSet("save", flag.ExitOnError)

	flags.StringVar(&cmdHandle, "name", "", "The name used to refer to the saved cmd")

	if err := flags.Parse(args); err != nil || cmdHandle == "" {
		fmt.Fprintf(os.Stderr, "Usage: save -name <name> <cmd> [<cmd args>, ...]\n")
		flags.PrintDefaults()
		os.Exit(2)
	}
}

func main() {
	// Run the selected sub-command.
	var status int
	switch subCmd {
	case deleteCommand:
		// TODO
	case listCommand:
		// TODO
	case runCommand:
		status = doCmdRun()
	case saveCommand:
		// TODO
	}
	os.Exit(status)
}
