package storage

import (
	"fmt"
	"wsfs-core/internal/server/config"

	"github.com/rs/zerolog/log"
)

type Users map[string]*User

func NewUsers(conf config.Server, anonymousUsername string) (users Users, anonymous *User, err error) {
	users = Users{}

	storages := map[string]*Storage{}
	storagesReadOnly := map[string]bool{}
	for _, st := range conf.Storages {
		if _, ok := storages[st.Id]; ok {
			if st.Id == "" {
				err = fmt.Errorf("default storage repeated")
			} else {
				err = fmt.Errorf("storage id '%s' repeated", st.Id)
			}
			return
		}

		storages[st.Id], err = NewStorage(&st)
		if err != nil {
			return
		}
		storagesReadOnly[st.Id] = st.ReadOnly
	}

	for _, us := range conf.Users {
		if _, ok := users[us.Name]; ok {
			err = fmt.Errorf("user '%s' repeated", us.Name)
			return
		}

		if us.Name == "" {
			err = fmt.Errorf("username can not be empty")
			return
		}

		if _, ok := storages[us.Storage]; !ok {
			err = fmt.Errorf("user '%s' referenced a storage that does not exist", us.Name)
			return
		}

		users[us.Name] = &User{
			Name:     us.Name,
			Password: []byte(us.SecretHash),
			Storage:  storages[us.Storage],
			ReadOnly: us.ReadOnly,
		}

		if storagesReadOnly[us.Storage] {
			users[us.Name].ReadOnly = true
		}
	}

	if conf.Anonymous.Enable {
		if _, ok := storages[conf.Anonymous.Storage]; !ok {
			err = fmt.Errorf("anonymous user referenced a storage that does not exist")
			return
		}
		anonymous = &User{
			ReadOnly: conf.Anonymous.ReadOnly,
			Storage:  storages[conf.Anonymous.Storage],
		}

		if storagesReadOnly[conf.Anonymous.Storage] {
			anonymous.ReadOnly = true
		}

		if _, ok := users[anonymousUsername]; ok {
			log.Warn().Str("Username", anonymousUsername).Msg("Anonymous's username used; it will not be considered anonymous")
		}
	}

	return
}
