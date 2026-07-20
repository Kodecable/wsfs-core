package hash

import (
	"errors"
	"fmt"
	"os"
	cmdexit "wsfs-core/internal/cmd/exit"

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
	RunE: func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			fmt.Printf("Password: ")
			password, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Printf("\n")
			if err != nil {
				return cmdexit.New(1, fmt.Errorf("unable to read stdin: %w", err))
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(password), int(hashCost))
			if err != nil {
				return cmdexit.New(1, fmt.Errorf("generate hash failed: %w", err))
			}
			fmt.Fprintln(os.Stdout, string(hash))

			return nil
		}

		var hashErrors []error
		for _, pw := range args {
			hash, err := bcrypt.GenerateFromPassword([]byte(pw), int(hashCost))
			if err != nil {
				hashErrors = append(hashErrors, fmt.Errorf("generate hash for %q: %w", pw, err))
			} else {
				fmt.Fprintln(os.Stdout, pw+":", string(hash))
			}
		}
		return cmdexit.New(1, errors.Join(hashErrors...))
	},
}

func init() {
	HashCmd.Flags().Uint8VarP(&hashCost, "cost", "c", uint8(bcrypt.DefaultCost), "Bcrypt hash cost")
}
