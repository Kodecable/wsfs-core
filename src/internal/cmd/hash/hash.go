package hash

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

var HashCmd = &cobra.Command{
	Use:   "hash [Password]...",
	Short: "Generate bcrypt hash(s) of the given password",
	Run: func(_ *cobra.Command, args []string) {
		if len(args) == 0 {
			password, err := term.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				fmt.Fprintln(os.Stderr, "Uable to read stdin:", err)
				os.Exit(1)
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Generate fail:", err)
				os.Exit(1)
			}
			fmt.Fprintln(os.Stdout, string(hash))

			return
		}

		for _, pw := range args {
			hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Generate fail:", pw, " err:", err)
			} else {
				fmt.Fprintln(os.Stdout, pw+":", string(hash))
			}
		}
	},
}
