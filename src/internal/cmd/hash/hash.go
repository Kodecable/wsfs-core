package hash

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

var HashCmd = &cobra.Command{
	Use:   "hash PASSWORD...",
	Short: "Generate bcrypt hash(s) of the given password",
	Args:  cobra.MinimumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
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
