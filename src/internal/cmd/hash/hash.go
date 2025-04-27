package hash

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

var (
	hashCost uint8
)

var HashCmd = &cobra.Command{
	Use:   "hash [Password]...",
	Short: "Generate bcrypt hash(s) of the given password",
	Long: `Generate bcrypt hash(s) of the given password
If no password is given as arguments, one will be read from stdin`,
	Run: func(_ *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Printf("Password: ")
			password, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Printf("\n")
			if err != nil {
				fmt.Fprintln(os.Stderr, "Uable to read stdin:", err)
				os.Exit(1)
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(password), int(hashCost))
			if err != nil {
				fmt.Fprintln(os.Stderr, "Generate fail:", err)
				os.Exit(1)
			}
			fmt.Fprintln(os.Stdout, string(hash))

			return
		}

		for _, pw := range args {
			hash, err := bcrypt.GenerateFromPassword([]byte(pw), int(hashCost))
			if err != nil {
				fmt.Fprintln(os.Stderr, "Generate fail:", pw, " err:", err)
			} else {
				fmt.Fprintln(os.Stdout, pw+":", string(hash))
			}
		}
	},
}

func init() {
	HashCmd.Flags().Uint8VarP(&hashCost, "cost", "c", uint8(bcrypt.DefaultCost), "Bcrypt hash cost")
}
