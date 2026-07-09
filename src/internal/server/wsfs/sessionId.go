package wsfs

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	mathrand "math/rand/v2"
	"sync"
	"time"
	"wsfs-core/internal/server/config"
)

const sessionIdByteLength = 16

type sessionIdSource interface {
	New() (string, error)
}

type cryptoSessionIdSource struct{}

func (cryptoSessionIdSource) New() (string, error) {
	return randomSessionId(rand.Reader)
}

type mathSessionIdSource struct {
	lock sync.Mutex
	rng  *mathrand.Rand
}

func newMathSessionIdSource() *mathSessionIdSource {
	seed := uint64(time.Now().UnixNano())
	return &mathSessionIdSource{
		rng: mathrand.New(mathrand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
	}
}

func (s *mathSessionIdSource) New() (string, error) {
	buf := make([]byte, sessionIdByteLength)

	s.lock.Lock()
	for i := range buf {
		buf[i] = byte(s.rng.Uint32())
	}
	s.lock.Unlock()

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func randomSessionId(r io.Reader) (string, error) {
	buf := make([]byte, sessionIdByteLength)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func setupSessionIdSource(c config.WSFS) sessionIdSource {
	if c.InsecureSessionIdMathRand {
		return newMathSessionIdSource()
	}
	return cryptoSessionIdSource{}
}
