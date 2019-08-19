# cmdsafe

cmdsafe is a small utility to store CLI commands with sensitive arguments, like passphrases, in an
encrypted format. These commands can then be executed with those arguments automatically appended -
plus any additional arguments passed to the run command - without having to manually copy the
passphrase from a separate password manager.

**Note: This was a small side project of mine that supports the desired functionality and is
currently not under active development!**

## Getting Started

### Installing

With [Go (Golang)](https://golang.org) installed on your machine, run the following command to
download, build and install the tool:

```
go get github.com/aleist/cmdsafe
```

## Usage

### Command Overview

``` 
$ cmdsafe 
Usage: cmdsafe [global flags ...] command [flags ...]

Global flags:
  -db path
        The database path (default "data.db")

The commands are:
  delete        delete a saved command
  list          list all saved commands
  print         print a command configuration to stdout
  run           run a saved command
  save          save a new or update an existing command
```

This shows the subcommands used to save, run and manage command configurations. Each command
configuration is given a name, which itself is not encrypted and should therefore not contain any
confidential information. This is used as a handle for the actual executable name (e.g. sshpass) and
arguments. This also means that multiple different configurations can be saved for a given
executable.

### Saving a command configuration

``` 
$ cmdsafe save
Usage: save [-r] -name <name> <cmd> [<cmd args> ...]
  -name string
        The name used to refer to the saved cmd
  -r    Replace existing entry with the given name
```

**Example**: Save a non-interactive, password-based SSH login using `sshpass` and `ssh` under the
name "server1":

```
$ cmdsafe save -name server1 sshpass -p secret ssh -p 2022 user@192.168.1.1
```

Note that it is more secure to use public key based authentication with SSH when possible.

### Running a command

``` 
$ cmdsafe run
Usage: run [-d] <cmd name> [<cmd args> ...]
  -d    Run the command in detached mode
```

Additional command arguments not stored with the command configuration can be passed to
`cmdsafe run` and  will be appended to the arguments from the configuration when the command is
invoked.

**Example**: Run the SSH command we saved above:

``` 
$ cmdsafe run server1
Enter password: 
```

### Listing all command configurations

``` 
$ cmdsafe list
server1
server2
```

### Printing a command

``` 
$ cmdsafe print
Usage: print <cmd name>
```

### Deleting a command

```
$ cmdsafe delete
Usage: delete <cmd name>
```

## Security

The following describes the steps used to secure each command configuration:

1. The command with its arguments is encrypted using a 256 bit AES-CTR (counter mode) stream cipher
with randomly generated encryption key and initialisation vector.
2. The unique encryption key from step 1 is itself encrypted with a key derived from the user
password using the memory-hard scrypt key derivation function. The encryption function is again
256 bit AES-CTR.
3. An SHA256 based HMAC using a second scrypt derived key from the user password is used to sign all
stored data to ensure integrity and authenticity: the two initialisation vectors,
the encryption key (from step 1) ciphertext, the command configuration ciphertext, and an identifier
for the encryption algorithm.

Note that this does not prevent the secret from showing up in the active process list and
potentially other places while the command is running, so this should not be used on systems where
that may be a concern. It only protects the command configuration and thus any secret contained
therein at rest.
