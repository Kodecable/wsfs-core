package wsfs

import (
	"context"
	"sync"
	"time"
	"wsfs-core/internal/server/config"
	"wsfs-core/internal/server/storage"
	"wsfs-core/internal/util"

	"github.com/rs/zerolog/log"
)

const (
	sessionInactiveScanPeriod = 3 * time.Minute
	sessionInactiveMaxCount   = 5
)

type SessionRegistry struct {
	lock     sync.Mutex
	idSource sessionIdSource
	sessions sync.Map
	ctx      context.Context

	stop context.CancelFunc
}

func NewSessionRegistry(c config.WSFS) *SessionRegistry {
	r := &SessionRegistry{
		idSource: setupSessionIdSource(c),
	}
	r.ctx, r.stop = context.WithCancel(context.Background())
	return r
}

func (r *SessionRegistry) Reconfigure(c config.WSFS) {
	r.lock.Lock()
	r.idSource = setupSessionIdSource(c)
	r.lock.Unlock()
}

func (r *SessionRegistry) Stop() {
	r.stop()
}

func (r *SessionRegistry) CollectInactiveSessions() {
	for {
		time.Sleep(sessionInactiveScanPeriod)
		existsSession := false
		r.sessions.Range(func(key, value any) bool {
			existsSession = true

			s := value.(*session)
			if s == nil {
				return true
			}

			if s.Lock.TryLock() {
				s.inactiveCount += 1
				if s.inactiveCount >= sessionInactiveMaxCount {
					r.delSession(key.(string))
					return true
				}
				s.Lock.Unlock()
			}
			return true
		})
		if r.ctx.Err() != nil && !existsSession {
			return
		}
	}
}

func (r *SessionRegistry) getSession(id string) *session {
	v, ok := r.sessions.Load(id)
	if !ok {
		return nil
	}
	return v.(*session)
}

func (r *SessionRegistry) newSession(username string, storage *storage.Storage, fsIds util.FsIds, featureOpts FeatureOptions) (string, error) {
	for {
		r.lock.Lock()
		idSource := r.idSource
		r.lock.Unlock()

		id, err := idSource.New()
		if err != nil {
			return "", err
		}
		if _, loaded := r.sessions.LoadOrStore(id, (*session)(nil)); loaded {
			continue
		}
		r.sessions.Store(id, newSession(r, id, username, storage, fsIds, featureOpts))
		log.Info().Str("Id", id).Msg("Session created")
		return id, nil
	}
}

func (r *SessionRegistry) delSession(id string) {
	if session := r.getSession(id); session != nil {
		session.clearFDs()
	}
	log.Info().Str("Id", id).Msg("Session destroyed")
	r.sessions.Delete(id)
}
