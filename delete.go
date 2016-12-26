// This file implements subcommand 'delete'.

package main

import "fmt"
import "github.com/boltdb/bolt"

// doCmdDelete executes subcommand 'delete', removing the key handle and its
// associated data from the DB.
func doCmdDelete(handle string) error {
	return accessDB(false, func(db *bolt.DB) error {
		if err := createBuckets(db); err != nil {
			return err
		}

		return db.Update(func(tx *bolt.Tx) error {
			cmdBucket := tx.Bucket([]byte(commandBucketName))
			// Tell the user if the key is not found in case it was a typo.
			if v := cmdBucket.Get([]byte(handle)); v == nil {
				return fmt.Errorf("%s not found", handle)
			}
			return cmdBucket.Delete([]byte(handle))
		})
	})
}
