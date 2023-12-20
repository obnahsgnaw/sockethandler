package sockethandler

import (
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/http"
	rpc2 "github.com/obnahsgnaw/rpc"
)

type Option func(s *Handler)

func (s *Handler) With(options ...Option) {
	for _, o := range options {
		if o != nil {
			o(s)
		}
	}
}

func DocServ(host url.Host, proxyPrefix string, provider func() ([]byte, error), public bool) Option {
	return func(s *Handler) {
		s.ds = NewDocServer(s.app.ID(), s.docConfig(host, proxyPrefix, provider, public))
	}
}
func DocServerIns(ins *http.PortedEngine, proxyPrefix string, provider func() ([]byte, error), public bool) Option {
	return func(s *Handler) {
		s.dsCus = true
		s.engin = ins
		s.ds = NewDocServerWithEngine(ins.Engine(), s.app.ID(), s.docConfig(ins.Host(), proxyPrefix, provider, public))
	}
}
func DocServerInsOrNew(ins *http.PortedEngine, newInsHost url.Host, proxyPrefix string, provider func() ([]byte, error), public bool) Option {
	if ins != nil {
		return DocServerIns(ins, proxyPrefix, provider, public)
	}

	return DocServ(newInsHost, proxyPrefix, provider, public)
}
func RpcIns(ins *rpc2.Server) Option {
	return func(s *Handler) {
		s.rs = ins
		s.rsCus = true
		s.host = ins.Host()
		s.initRegInfo()
	}
}
func RpcInsOrNew(ins *rpc2.Server, host url.Host) Option {
	if ins != nil {
		return RpcIns(ins)
	}
	return Rpc(host)
}
func Rpc(host url.Host) Option {
	return func(s *Handler) {
		s.host = host
		s.initRegInfo()
	}
}
