package common

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
)

func SudoCheck(_ *cobra.Command, _ []string) error {
	id := os.Geteuid()
	if id != 0 {
		return errors.New("containerlab requires sudo privileges to run")
	}
	return nil
}
