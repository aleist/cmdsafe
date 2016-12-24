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
	progName string  // The name of this executable.
	subCmd   command // The active sub-command.

	dbPath    = "data.db"   // The path to the DB file.
	cmdHandle string        // The handle for the external cmd.
	cmdConfig *data.Command // The external command data.

	saveConfig *saveOptions // saveCommand specific options.
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

// Database constants.
const (
	configBucketName  = "config"  // The config bucket.
	commandBucketName = "command" // The command data bucket.
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
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: delete <cmd name>\n")
		os.Exit(2)
	}

	cmdHandle = args[0]
}

// initCmdRun parses arguments specific to sub-command 'run'.
func initCmdRun(args []string) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: run <cmd name>\n")
		os.Exit(2)
	}

	cmdHandle = args[0]
}

// initCmdSave parses arguments specific to sub-command 'save'.
func initCmdSave(args []string) {
	flags := flag.NewFlagSet("save", flag.ExitOnError)

	saveConfig = &saveOptions{}

	flags.StringVar(&cmdHandle, "name", "", "The name used to refer to the saved cmd")
	flags.BoolVar(&saveConfig.Replace, "r", false, "Replace existing entry with the given name")

	err := flags.Parse(args)
	cmdArgs := flags.Args()
	if err != nil || cmdHandle == "" || len(cmdArgs) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: save [-r] -name <name> <cmd> [<cmd args>, ...]\n")
		flags.PrintDefaults()
		os.Exit(2)
	}

	// Init the external command struct.
	cmdConfig = &data.Command{}
	cmdConfig.Name = cmdArgs[0]
	if len(cmdArgs) > 1 {
		cmdConfig.Args = cmdArgs[1:]
	}
}

func main() {
	// Run the selected sub-command.
	var status int
	var err error
	switch subCmd {
	case deleteCommand:
		// TODO
	case listCommand:
		// TODO
	case runCommand:
		status, err = doCmdRun(cmdHandle)
	case saveCommand:
		err = doCmdSave(cmdHandle, saveConfig)
	}
	if err != nil {
		log.Print(err)
	}

	if status == 0 && err != nil {
		os.Exit(1)
	}
	os.Exit(status)
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
