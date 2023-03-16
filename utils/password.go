package utils

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

func ReadPasswordFromTerminal() (string, error) {
	fmt.Print("password: ")
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	fmt.Println()
	return string(pass), nil
}
