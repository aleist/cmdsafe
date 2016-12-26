package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"bitbucket.org/aleist/cmdsafe/protobuf/data"
	"github.com/boltdb/bolt"
)

var (
	progName string      // The name of this executable.
	dbPath   = "data.db" // The path to the DB file.
)

// command is the type of a valid subcommand.
type command string

// Valid subcommands.
const (
	deleteCommand command = "delete"
	listCommand   command = "list"
	runCommand    command = "run"
	saveCommand   command = "save"
)

// Database constants.
const (
	configBucketName  = "config"  // The config bucket.
	commandBucketName = "command" // The command data bucket.
)

func main() {
	// Parse global arguments.
	subcmd, subargs := parseArgs()

	// Parse subcommand arguments and run it.
	var status int
	var err error
	switch subcmd {
	case deleteCommand:
		_ = parseArgsCmdDelete(subargs)
		// TODO
	case listCommand:
		// No arguments to parse.
		err = doCmdList()
	case runCommand:
		cmdHandle := parseArgsCmdRun(subargs)
		status, err = doCmdRun(cmdHandle)
	case saveCommand:
		cmdHandle, cmdData, config := parseArgsCmdSave(subargs)
		err = doCmdSave(cmdHandle, cmdData, config)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", subcmd)
		flag.Usage()
		os.Exit(2)
	}
	if err != nil {
		log.Print(err)
	}

	if status == 0 && err != nil {
		os.Exit(1)
	}
	os.Exit(status)
}

// parseArgs parses the global command line arguments and returns the specified
// subcommand and its unparsed arguments.
func parseArgs() (command, []string) {
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

	// Extract the subcommand.
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: missing command name\n")
		flag.Usage()
		os.Exit(2)
	}
	return command(args[0]), args[1:]
}

// parseArgsCmdDelete parses arguments specific to subcommand 'delete'. Returns
// the handle for the external command to be deleted.
func parseArgsCmdDelete(args []string) (cmdHandle string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: delete <cmd name>\n")
		os.Exit(2)
	}
	return args[0]
}

// parseArgsCmdRun parses arguments specific to subcommand 'run'. Returns the
// handle for the external command to be run.
func parseArgsCmdRun(args []string) (cmdHandle string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: run <cmd name>\n")
		os.Exit(2)
	}
	return args[0]
}

// parseArgsCmdSave parses arguments specific to subcommand 'save'. Returns the
// handle for the external command to be saved, its data and the save config.
func parseArgsCmdSave(args []string) (cmdHandle string, cmdData *data.Command, config *saveOptions) {
	flags := flag.NewFlagSet("save", flag.ExitOnError)

	config = &saveOptions{}

	flags.StringVar(&cmdHandle, "name", "", "The name used to refer to the saved cmd")
	flags.BoolVar(&config.Replace, "r", false, "Replace existing entry with the given name")

	err := flags.Parse(args)
	cmdArgs := flags.Args()
	if err != nil || cmdHandle == "" || len(cmdArgs) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: save [-r] -name <name> <cmd> [<cmd args>, ...]\n")
		flags.PrintDefaults()
		os.Exit(2)
	}

	// Init the external command struct.
	cmdData = &data.Command{}
	cmdData.Name = cmdArgs[0]
	if len(cmdArgs) > 1 {
		cmdData.Args = cmdArgs[1:]
	}

	return cmdHandle, cmdData, config
}

// accessDB opens the database in either readwrite or readonly mode and passes
// the instance to function fn. The database is closed and all resources
// released when fn returns.
//
// The returned error may be from the database access or from fn, whichever
// occurs first.
func accessDB(readonly bool, fn func(*bolt.DB) error) (err error) {
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{ReadOnly: readonly, Timeout: 5 * time.Second})
	if err != nil {
		return err
	}
	defer func() {
		if e := db.Close(); err == nil { // Return the first error encountered.
			err = e
		}
	}()

	return fn(db)
}

// createBuckets creates the top-level buckets in db if they do not exist.
func createBuckets(db *bolt.DB) error {
	return db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(commandBucketName))
		if err == nil {
			_, err = tx.CreateBucketIfNotExists([]byte(configBucketName))
		}
		return err
	})
}
