// This file implements subcommand 'list'.

package main

import (
	"fmt"
	"os"

	"github.com/boltdb/bolt"
)

// doCmdList executes subcommand 'list', printing the handles of all stored
// command entries.
func doCmdList() error {
	// Check if the DB file is readable.
	info, err := os.Stat(dbPath)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("cannot read database: %v", err)
	}

	// Retrieve and print handles.
	handles, err := commandList()
	if err != nil {
		return err
	}
	for _, v := range handles {
		fmt.Println(v)
	}

	return nil
}

// commandList queries the DB and returns a list of all the command handles
// stored in it.
func commandList() ([]string, error) {
	var handles []string
	err := accessDB(true, func(db *bolt.DB) error {
		return db.View(func(tx *bolt.Tx) error {
			cmdBucket := tx.Bucket([]byte(commandBucketName))
			if cmdBucket == nil {
				return nil // DB structure has not been created, treat as empty.
			}

			// Iterate over keys.
			c := cmdBucket.Cursor()
			for k, _ := c.First(); k != nil; k, _ = c.Next() {
				// Converting the key to string also copies it, do not return []byte.
				handles = append(handles, string(k))
			}
			return nil
		})
	})

	return handles, err
}
