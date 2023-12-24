package impl

import (
	handlerv1 "github.com/obnahsgnaw/socketapi/gen/handler/v1"
	"github.com/obnahsgnaw/sockethandler/service/action"
)

type ManagerProvider struct {
	builder  func() *action.Manager
	provider map[handlerv1.SocketType]*action.Manager
}

func NewManagerProvider(builder func() *action.Manager) *ManagerProvider {
	return &ManagerProvider{
		builder:  builder,
		provider: make(map[handlerv1.SocketType]*action.Manager),
	}
}

func (s *ManagerProvider) GetManager(typ handlerv1.SocketType) *action.Manager {
	if _, ok := s.provider[typ]; !ok {
		s.provider[typ] = s.builder()
	}
	return s.provider[typ]
}
