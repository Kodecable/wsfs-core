package server

import (
	"errors"
	"net/http"
	"wsfs-core/internal/server/storage"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrBadHttpAuthHeader = errors.New("bad http header 'auth'")
	ErrUserNotExists     = errors.New("user not exists")
	ErrHashMismatch      = errors.New("password hash mismatch")
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

func AuthUser(users map[string]User, username, password string) (*User, error) {
	var user User
	var ok bool

	if user, ok = users[username]; !ok {
		log.Info().Str("Name", username).Msg("User not exists")
		return nil, ErrUserNotExists
	}

	return &user, user.CheckPassword(password)
}

func HttpBasciAuth(users map[string]User, req *http.Request) (*User, error) {
	username, password, ok := req.BasicAuth()
	if !ok {
		return nil, ErrBadHttpAuthHeader
	}
	return AuthUser(users, username, password)
}
