package wsfs

import (
	"math/rand/v2"

	"github.com/sqids/sqids-go"
)

const (
	sessionIdMinLength = 13
	sessionIdRunes     = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

func randomSessionIdAlphabet() string {
	s := []rune(sessionIdRunes)
	rand.Shuffle(len(s), func(i, j int) {
		s[i], s[j] = s[j], s[i]
	})
	return string(s)
}

func setupIder() (*sqids.Sqids, error) {
	return sqids.New(sqids.Options{
		MinLength: sessionIdMinLength,
		Alphabet:  randomSessionIdAlphabet(),
	})
}
