package cli

import "fmt"

var Version = "dev"

func VersionCommand(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("version command does not accept arguments")
	}
	fmt.Println(Version)
	return nil
}
