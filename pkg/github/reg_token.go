package github

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

type regToken struct {
	value   string
	renewAt time.Time
}

type RegistrationTokenStore struct {
	logger *zap.Logger
	target Target

	token *regToken
	lock  *sync.RWMutex
	group *singleflight.Group
}

func NewRegistrationTokenStore(logger *zap.Logger, target Target) *RegistrationTokenStore {
	return &RegistrationTokenStore{
		logger: logger.Named("reg-token"),
		target: target,

		token: &regToken{},
		lock:  new(sync.RWMutex),
		group: new(singleflight.Group),
	}
}

func (s *RegistrationTokenStore) Get(ctx context.Context) (string, error) {
	token := s.get(ctx)
	if time.Now().Before(token.renewAt) {
		return token.value, nil
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()

	case result := <-s.group.DoChan("", s.renew):
		if result.Err != nil {
			return "", result.Err
		}
		return result.Val.(*regToken).value, nil
	}
}

func (s *RegistrationTokenStore) get(ctx context.Context) *regToken {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.token
}

func (s *RegistrationTokenStore) renew() (interface{}, error) {
	s.logger.Info("fetching token")

	token, err := s.target.GetRegistrationToken(context.TODO())
	if err != nil {
		s.logger.Warn("fetch failed", zap.Error(err))
		return nil, err
	}

	expireAt := token.GetExpiresAt().Time
	s.logger.Info("token fetched", zap.Time("expireAt", expireAt))

	rt := &regToken{
		value:   token.GetToken(),
		renewAt: expireAt.Add(-30 * time.Minute),
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	s.token = rt
	return s.token, nil
}
