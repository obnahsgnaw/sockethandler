package impl

import (
	"github.com/obnahsgnaw/sockethandler/service/action"
)

type ManagerProvider struct {
	builder  func() *action.Manager
	provider map[string]*action.Manager
}

func NewManagerProvider(builder func() *action.Manager) *ManagerProvider {
	return &ManagerProvider{
		builder:  builder,
		provider: make(map[string]*action.Manager),
	}
}

func (s *ManagerProvider) GetManager(businessChannel string) *action.Manager {
	if _, ok := s.provider[businessChannel]; !ok {
		s.provider[businessChannel] = s.builder()
	}
	return s.provider[businessChannel]
}
