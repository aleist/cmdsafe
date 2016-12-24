// This file implements sub-command 'save'.

package main

import (
	"fmt"

	"github.com/boltdb/bolt"
	"github.com/golang/protobuf/proto"
)

type saveOptions struct {
	Replace bool // Replace existing value.
}

// doCmdSave executes sub-command 'save'.
func doCmdSave() error {
	// Serialise the Command proto.
	_, err := proto.Marshal(cmdConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal the Command config: %v", err)
	}

	return nil
}

func writeCommand(db *bolt.DB) error {

	return nil
}
