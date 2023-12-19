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

func DocServ(host url.Host, proxyPrefix string, provider func() ([]byte, error), public bool, projPrefixed bool) Option {
	return func(s *Handler) {
		s.WithDocServer(host, proxyPrefix, provider, public, projPrefixed)
	}
}
func DocServerIns(ins *http.PortedEngine, proxyPrefix string, provider func() ([]byte, error), public bool) Option {
	return func(s *Handler) {
		s.WithDocServerIns(ins, proxyPrefix, provider, public)
	}
}
func DocServerInsOrNew(ins *http.PortedEngine, newInsHost url.Host, proxyPrefix string, provider func() ([]byte, error), public bool, projPrefixed bool) Option {
	if ins != nil {
		return DocServerIns(ins, proxyPrefix, provider, public)
	}

	return DocServ(newInsHost, proxyPrefix, provider, public, projPrefixed)
}
func RpcIns(ins *rpc2.Server) Option {
	return func(s *Handler) {
		s.WithRpcIns(ins)
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
		s.WithRpc(host)
	}
}
