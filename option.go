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

func RpcIgRun(ig bool) Option {
	return func(s *Handler) {
		s.rpcIgRun = ig
	}
}

func DocServ(ins *http.Http, provider func() ([]byte, error), public, igEngineRun bool) Option {
	return func(s *Handler) {
		if ins != nil {
			s.docIgRun = igEngineRun
			s.docServer = NewDocServer(ins, s.app.ID(), s.docConfig(provider, public))
			s.engin = ins
		}
	}
}
