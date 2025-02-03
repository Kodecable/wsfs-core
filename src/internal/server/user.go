package server

import (
	"errors"
	"net/http"
	"wsfs-core/internal/server/storage"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

const (
	anonymousUsername = "anonymous"
)

var (
	ErrBadHttpAuthHeader   = errors.New("bad http auth header")
	ErrUserNotExists       = errors.New("user not exists")
	ErrHashMismatch        = errors.New("password hash mismatch")
	ErrAuthHeaderNotExists = errors.New("http auth header not exists")
	ErrAnonymous           = errors.New("anonymous user")
)

type User struct {
	Name     string
	Password string
	Storage  *storage.Storage
}

func (u *User) CheckPassword(password string) (err error) {
	err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))

	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrHashMismatch
	}
	return err
}

func authUser(users map[string]User, username, password string) (*User, error) {
	var user User
	var ok bool

	if user, ok = users[username]; !ok {
		if username == anonymousUsername {
			return nil, ErrAnonymous
		}
		log.Info().Str("Name", username).Msg("User not exists")
		return nil, ErrUserNotExists
	}

	return &user, user.CheckPassword(password)
}

func httpBasciAuth(users map[string]User, req *http.Request) (*User, error) {
	if req.Header.Get("Authorization") == "" {
		return nil, ErrAuthHeaderNotExists
	}

	username, password, ok := req.BasicAuth()
	if !ok {
		return nil, ErrBadHttpAuthHeader
	}

	return authUser(users, username, password)
}
