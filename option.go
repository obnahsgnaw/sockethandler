package sockethandler

import (
	"github.com/obnahsgnaw/http"
)

type Option func(s *Handler)

func with(s *Handler, options ...Option) {
	for _, o := range options {
		if o != nil {
			o(s)
		}
	}
}

func DocServ(ins *http.Http, provider func() ([]byte, error), public bool) Option {
	return func(s *Handler) {
		if ins != nil {
			s.docServer = NewDocServer(ins, s.app.Cluster().Id(), s.docConfig(provider, public))
			s.engin = ins
		}
	}
}

func WatchChannelGateway(channel ...string) Option {
	return func(s *Handler) {
		for _, ch := range channel {
			if _, ok := s.gateways[ch]; !ok {
				gw, reg := s.initChannelGateway(ch)
				s.gateways[ch] = gw
				s.watchGwRegInfos[ch] = reg
			}
		}
	}
}
