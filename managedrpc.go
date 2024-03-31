package sockethandler

import (
	"github.com/obnahsgnaw/application"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/http/listener"
	"github.com/obnahsgnaw/rpc"
	handlerv1 "github.com/obnahsgnaw/socketapi/gen/handler/v1"
	"github.com/obnahsgnaw/sockethandler/service/action"
	"github.com/obnahsgnaw/sockethandler/service/proto/impl"
	"github.com/obnahsgnaw/sockethandler/sockettype"
)

type ManagedRpc struct {
	s *rpc.Server
	m *impl.ManagerProvider
}

func (s *ManagedRpc) Server() *rpc.Server {
	return s.s
}

func (s *ManagedRpc) Manager() *impl.ManagerProvider {
	return s.m
}

func InitRpc(s *rpc.Server) *ManagedRpc {
	am := impl.NewManagerProvider(func() *action.Manager {
		return action.NewManager()
	})
	s.RegisterService(rpc.ServiceInfo{
		Desc: handlerv1.HandlerService_ServiceDesc,
		Impl: impl.NewHandlerService(am),
	})
	return &ManagedRpc{
		s: s,
		m: am,
	}
}

func NewRpc(app *application.Application, module, subModule string, et endtype.EndType, sct sockettype.SocketType, lr *listener.PortedListener, p *rpc.PServer, o ...rpc.Option) *ManagedRpc {
	id := module + "-" + subModule
	s := rpc.New(app, lr, id, utils.ToStr(sct.ToServerType().String(), "-", id, "-rpc"), et, p, o...)
	am := impl.NewManagerProvider(func() *action.Manager {
		return action.NewManager()
	})
	s.RegisterService(rpc.ServiceInfo{
		Desc: handlerv1.HandlerService_ServiceDesc,
		Impl: impl.NewHandlerService(am),
	})
	return &ManagedRpc{
		s: s,
		m: am,
	}
}
