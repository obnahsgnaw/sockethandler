package sockethandler

import (
	"errors"
	"github.com/obnahsgnaw/application"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/logging/logger"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/application/regtype"
	"github.com/obnahsgnaw/application/servertype"
	"github.com/obnahsgnaw/application/service/regCenter"
	rpc2 "github.com/obnahsgnaw/rpc"
	"github.com/obnahsgnaw/rpc/pkg/rpc"
	"github.com/obnahsgnaw/sockethandler/service/action"
	"github.com/obnahsgnaw/sockethandler/service/codec"
	handlerv1 "github.com/obnahsgnaw/sockethandler/service/proto/gen/handler/v1"
	"github.com/obnahsgnaw/sockethandler/service/proto/impl"
	"github.com/obnahsgnaw/sockethandler/sockettype"
	"go.uber.org/zap"
	"strings"
)

type Handler struct {
	id      string
	name    string
	st      servertype.ServerType
	sct     sockettype.SocketType
	et      endtype.EndType
	host    url.Host
	app     *application.Application
	am      *action.Manager
	rpm     *rpc.Manager
	rs      *rpc2.Server
	logger  *zap.Logger
	ds      *DocServer
	regInfo *regCenter.RegInfo // actions
	err     error
}

func New(app *application.Application, module, subModule, name string, et endtype.EndType, st sockettype.SocketType, rpcHost url.Host) *Handler {
	s := &Handler{
		id:   module + "-" + subModule,
		name: name,
		st:   servertype.Hdl,
		sct:  st,
		et:   et,
		app:  app,
		am:   action.NewManager(),
		rpm:  rpc.NewManager(),
		host: rpcHost,
	}
	s.logger, s.err = logger.New(utils.ToStr("Hdl[", s.et.String(), "-", s.sct.String(), "-", module+"-"+subModule, "]"), s.app.LogConfig(), s.app.Debugger().Debug())
	s.regInfo = &regCenter.RegInfo{
		AppId:   s.app.ID(),
		RegType: regtype.Rpc,
		ServerInfo: regCenter.ServerInfo{
			Id:      s.id,
			Name:    s.name,
			Type:    s.st.String(),
			EndType: s.et.String(),
		},
		Host:      s.host.String(),
		Val:       s.host.String(),
		Ttl:       s.app.RegTtl(),
		KeyPreGen: regCenter.ActionRegKeyPrefixGenerator(),
	}

	ss := rpc2.New(app, s.id, utils.ToStr(s.st.String(), "-", s.id, "-rpc"), s.et, s.host, rpc2.Parent(s))
	ss.RegisterService(rpc2.ServiceInfo{
		Desc: handlerv1.HandlerService_ServiceDesc,
		Impl: impl.NewHandlerService(s.am, s.logger.Named("handle")),
	})
	s.rs = ss

	return s
}

// ID return the server id
func (s *Handler) ID() string {
	return s.id
}

// Name return the server name
func (s *Handler) Name() string {
	return s.name
}

// Type return the server type
func (s *Handler) Type() servertype.ServerType {
	return s.st
}

// EndType return the server end type
func (s *Handler) EndType() endtype.EndType {
	return s.et
}

// SocketType return the server socket type
func (s *Handler) SocketType() sockettype.SocketType {
	return s.sct
}

// Logger return the logger
func (s *Handler) Logger() *zap.Logger {
	return s.logger
}

// WithDocServer with doc server
func (s *Handler) WithDocServer(port int, path string, provider func() ([]byte, error)) {
	config := &DocConfig{
		id:      s.id,
		endType: s.et,
		Origin: url.Origin{
			Protocol: url.HTTP,
			Host: url.Host{
				Ip:   s.host.Ip,
				Port: port,
			},
		},
		RegTtl: s.app.RegTtl(),
		Doc: DocItem{
			socketType: s.sct,
			Path:       path,
			Title:      s.name,
			Provider:   provider,
		},
	}
	s.ds = NewDocServer(s.app.ID(), config)
	s.debug("withed handler doc server")
}

// Release resource
func (s *Handler) Release() {
	if s.rs != nil {
		s.rs.Release()
	}
	if s.app.Register() != nil {
		_ = s.register(s.app.Register(), false)
	}
	_ = s.logger.Sync()
}

// Run start run
func (s *Handler) Run(failedCb func(error)) {
	if s.err != nil {
		failedCb(s.err)
		return
	}
	if s.st == "" {
		failedCb(errors.New(s.msg("type not support")))
		return
	}
	if s.app.Register() != nil {
		if err := s.register(s.app.Register(), true); err != nil {
			failedCb(utils.NewWrappedError(s.msg("register failed"), err))
		}
	}
	if s.ds != nil {
		docDesc := utils.ToStr("doc[", s.ds.config.Origin.Host.String(), "] ")
		s.logger.Info(docDesc + "start and serving...")
		s.logger.Info(docDesc + "socket doc url=" + s.ds.DocUrl())
		s.ds.SyncStart(failedCb)
	}
	if s.rs != nil {
		s.rs.Run(failedCb)
	}
}

// Listen action
func (s *Handler) Listen(act codec.Action, structure action.DataStructure, handler action.Handler) {
	s.am.RegisterHandler(act, structure, handler)
}

func (s *Handler) register(register regCenter.Register, reg bool) error {
	if s.ds != nil {
		if reg {
			if err := s.app.DoRegister(s.ds.RegInfo()); err != nil {
				return err
			}
		} else {
			if err := s.app.DoUnregister(s.ds.RegInfo()); err != nil {
				return err
			}
		}
	}
	err := s.am.RangeHandlerActions(func(act codec.Action) error {
		prefix := s.regInfo.Prefix()
		key := strings.TrimPrefix(strings.Join([]string{prefix, s.regInfo.ServerInfo.Id, s.regInfo.Host, act.Id.String()}, "/"), "/")
		if reg {
			if err := register.Register(s.app.Context(), key, act.Name, s.regInfo.Ttl); err != nil {
				return err
			}
		} else {
			if err := register.Unregister(s.app.Context(), key); err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

func (s *Handler) debug(msg string) {
	if s.app.Debugger().Debug() {
		s.logger.Debug(msg)
	}
}

func (s *Handler) msg(msg ...string) string {
	return utils.ToStr("Socket Handler[", s.name, "] ", utils.ToStr(msg...))
}
