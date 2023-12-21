package sockethandler

import (
	"errors"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/http"
	rpc2 "github.com/obnahsgnaw/rpc"
	handlerv1 "github.com/obnahsgnaw/socketapi/gen/handler/v1"
	"github.com/obnahsgnaw/sockethandler/service/action"
	"github.com/obnahsgnaw/sockethandler/service/proto/impl"
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
		s.engin = http.NewPortedEngine(s.ds.engine, host)
	}
}
func DocServerIns(ins *http.PortedEngine, proxyPrefix string, provider func() ([]byte, error), public bool) Option {
	return func(s *Handler) {
		s.dsCus = true
		s.ds = NewDocServerWithEngine(ins.Engine(), s.app.ID(), s.docConfig(ins.Host(), proxyPrefix, provider, public))
		s.engin = ins
	}
}
func DocServerInsOrNew(ins *http.PortedEngine, newInsHost url.Host, proxyPrefix string, provider func() ([]byte, error), public bool) Option {
	if ins != nil {
		return DocServerIns(ins, proxyPrefix, provider, public)
	}

	return DocServ(newInsHost, proxyPrefix, provider, public)
}
func RpcIns(ins *rpc2.Server, am *action.Manager) Option {
	return func(s *Handler) {
		s.rs = ins
		s.rsCus = true
		s.host = ins.Host()
		s.initRegInfo()
		if am != nil {
			s.am = am
		}
	}
}
func RpcInsOrNew(ins *rpc2.Server, am *action.Manager, host url.Host) Option {
	if ins != nil {
		return RpcIns(ins, am)
	}
	return Rpc(host)
}
func Rpc(host url.Host) Option {
	return func(s *Handler) {
		s.host = host
		if s.host.Port <= 0 {
			s.addErr(errors.New("handler rpc port required"))
			return
		}
		s.rs = rpc2.New(s.app, s.id, utils.ToStr(s.st.String(), "-", s.id, "-rpc"), s.et, s.host, rpc2.Parent(s))
		s.rs.RegisterService(rpc2.ServiceInfo{
			Desc: handlerv1.HandlerService_ServiceDesc,
			Impl: impl.NewHandlerService(s.am),
		})
		s.initRegInfo()
	}
}
