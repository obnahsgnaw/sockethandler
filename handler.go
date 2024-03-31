package sockethandler

import (
	"context"
	"errors"
	"github.com/obnahsgnaw/application"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/logging/logger"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/application/regtype"
	"github.com/obnahsgnaw/application/servertype"
	"github.com/obnahsgnaw/application/service/regCenter"
	"github.com/obnahsgnaw/http"
	"github.com/obnahsgnaw/rpc/pkg/rpcclient"
	"github.com/obnahsgnaw/sockethandler/service/action"
	"github.com/obnahsgnaw/sockethandler/service/proto/impl"
	"github.com/obnahsgnaw/sockethandler/sockettype"
	"github.com/obnahsgnaw/socketutil/codec"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"strconv"
	"strings"
)

type Handler struct {
	app          *application.Application
	id           string
	module       string
	subModule    string
	name         string
	endType      endtype.EndType
	serverType   servertype.ServerType
	socketType   sockettype.SocketType
	rpcServer    *ManagedRpc
	logger       *zap.Logger
	logCnf       *logger.Config
	engin        *http.Http
	docServer    *DocServer
	regInfo      *regCenter.RegInfo // actions
	tcpGwRegInfo *regCenter.RegInfo // tcp gateway
	wssGwRegInfo *regCenter.RegInfo // wss gateway
	tcpGw        *impl.Gateway
	wssGw        *impl.Gateway
	rpcIgRun     bool
	docIgRun     bool
	errs         []error
	flbNum       int
	actListeners []func(manager *action.Manager)
}

func New(app *application.Application, rps *ManagedRpc, module, subModule, name string, et endtype.EndType, sct sockettype.SocketType, o ...Option) *Handler {
	s := &Handler{
		id:         module + "-" + subModule,
		module:     module,
		subModule:  subModule,
		name:       name,
		app:        app,
		endType:    et,
		serverType: sct.ToHandlerType(),
		socketType: sct,
		rpcServer:  rps,
		tcpGw:      impl.NewGateway(app.Context(), rpcclient.NewManager()),
		wssGw:      impl.NewGateway(app.Context(), rpcclient.NewManager()),
	}
	if s.serverType == "" {
		s.addErr(s.handlerError("type invalid", errors.New("type not support")))
	}
	s.initLogger()
	s.initRegInfo()
	s.tcpGw.Manager().RegisterAfterHandler(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, err error, opts ...grpc.CallOption) {
		if err != nil {
			s.logger.Error(utils.ToStr("rpc call tcp-gateway[", method, "] failed, ", err.Error()), zap.Any("req", req), zap.Any("resp", reply))
		} else {
			s.logger.Debug(utils.ToStr("rpc call tcp-gateway[", method, "] success"), zap.Any("req", req), zap.Any("resp", reply))
		}
	})
	s.wssGw.Manager().RegisterAfterHandler(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, err error, opts ...grpc.CallOption) {
		if err != nil {
			s.logger.Error(utils.ToStr("rpc call wss-gateway[", method, "] failed, ", err.Error()), zap.Any("req", req), zap.Any("resp", reply))
		} else {
			s.logger.Debug(utils.ToStr("rpc call wss-gateway[", method, "] success"), zap.Any("req", req), zap.Any("resp", reply))
		}
	})
	s.tcpGwRegInfo = &regCenter.RegInfo{
		AppId:   app.Cluster().Id(),
		RegType: regtype.Rpc,
		ServerInfo: regCenter.ServerInfo{
			Id:      sockettype.TCP.String() + "-gateway",
			Name:    "",
			Type:    servertype.Tcp.String(),
			EndType: s.endType.String(),
		},
	}
	s.wssGwRegInfo = &regCenter.RegInfo{
		AppId:   app.Cluster().Id(),
		RegType: regtype.Rpc,
		ServerInfo: regCenter.ServerInfo{
			Id:      sockettype.WSS.String() + "-gateway",
			Name:    "",
			Type:    servertype.Wss.String(),
			EndType: s.endType.String(),
		},
	}
	with(s, o...)
	return s
}

func (s *Handler) ID() string {
	return s.id
}

// Name return the server name
func (s *Handler) Name() string {
	return s.name
}

// Type return the server type
func (s *Handler) Type() servertype.ServerType {
	return s.serverType
}

// EndType return the server end type
func (s *Handler) EndType() endtype.EndType {
	return s.endType
}

// SocketType return the server socket type
func (s *Handler) SocketType() sockettype.SocketType {
	return s.socketType
}

// Run start run
func (s *Handler) Run(failedCb func(error)) {
	if s.errs != nil {
		failedCb(s.errs[0])
		return
	}
	s.logger.Info("init start...")
	if s.docServer != nil {
		s.logger.Debug("doc server enabled")
		s.logger.Info("doc url=" + s.docServer.DocUrl())
		if !s.docIgRun {
			s.logger.Info(utils.ToStr("doc server[", s.docServer.engine.Host(), "] start and serving..."))
			s.docServer.SyncStart(failedCb)
		}
		if s.app.Register() != nil {
			if err := s.app.DoRegister(s.docServer.RegInfo(), func(msg string) {
				s.logger.Debug(msg)
			}); err != nil {
				failedCb(err)
				return
			}
		}
	}
	s.logger.Debug("handler watch start")
	if err := s.watch(s.app.Register()); err != nil {
		failedCb(err)
		return
	}
	for _, h := range s.actListeners {
		h(s.rpcServer.Manager().GetManager(s.socketType.ToHandlerSocketType()))
	}
	s.logger.Info("listen action initialized")
	if s.app.Register() != nil {
		s.logger.Debug("action register start")
		if err := s.register(s.app.Register(), true); err != nil {
			failedCb(s.handlerError("register failed", err))
		}
		s.logger.Debug("action registered")
	}
	s.logger.Info("initialized")
	if !s.rpcIgRun {
		s.logger.Info(utils.ToStr("server[", s.rpcServer.Server().Host().String(), "] start and serving..."))
		s.rpcServer.Server().Run(failedCb)
	}
}

// Release resource
func (s *Handler) Release() {
	if s.rpcServer != nil {
		s.rpcServer.s.Release()
	}
	if s.app.Register() != nil {
		if s.docServer != nil {
			_ = s.app.DoUnregister(s.docServer.RegInfo(), func(msg string) {
				if s.logger != nil {
					s.logger.Debug(msg)
				}
			})
		}
		_ = s.register(s.app.Register(), false)
	}
	if s.logger != nil {
		s.logger.Info("released")
		_ = s.logger.Sync()
	}
}

func (s *Handler) TcpGateway() *impl.Gateway {
	return s.tcpGw
}

func (s *Handler) WssGateway() *impl.Gateway {
	return s.wssGw
}

func (s *Handler) ActionManager() *impl.ManagerProvider {
	return s.rpcServer.Manager()
}

// Logger return the logger
func (s *Handler) Logger() *zap.Logger {
	return s.logger
}

func (s *Handler) LogConfig() *logger.Config {
	return s.logCnf
}

func (s *Handler) Rpc() *ManagedRpc {
	return s.rpcServer
}

func (s *Handler) Engine() *http.Http {
	return s.engin
}

// Listen action
func (s *Handler) Listen(act codec.Action, structure action.DataStructure, handler action.Handler) {
	s.actListeners = append(s.actListeners, func(manager *action.Manager) {
		if _, _, _, ok := manager.GetHandler(act.Id); ok {
			panic("action[" + act.String() + "] already listened.")
		}
		manager.RegisterHandler(act, structure, handler)
		s.logger.Debug("listened action:" + act.Name)
	})
}

func (s *Handler) docConfig(provider func() ([]byte, error), public bool) *DocConfig {
	return &DocConfig{
		id:       s.id,
		endType:  s.endType,
		servType: s.serverType,
		RegTtl:   s.app.RegTtl(),
		Doc: DocItem{
			socketType: s.socketType,
			Title:      s.name,
			Public:     public,
			Provider:   provider,
			Module:     s.module,
			SubModule:  s.subModule,
		},
	}
}

func (s *Handler) initRegInfo() {
	s.regInfo = &regCenter.RegInfo{
		AppId:   s.app.Cluster().Id(),
		RegType: regtype.Rpc,
		ServerInfo: regCenter.ServerInfo{
			Id:      s.id,
			Name:    s.name,
			Type:    s.serverType.String(),
			EndType: s.endType.String(),
		},
		Host:      s.rpcServer.Server().Host().String(),
		Val:       s.rpcServer.Server().Host().String(),
		Ttl:       s.app.RegTtl(),
		KeyPreGen: regCenter.ActionRegKeyPrefixGenerator(),
	}
}

func (s *Handler) handlerError(msg string, err error) error {
	return utils.TitledError(utils.ToStr("handler[", s.name, "] error"), msg, err)
}

func (s *Handler) register(register regCenter.Register, reg bool) error {
	return s.rpcServer.Manager().GetManager(s.socketType.ToHandlerSocketType()).RangeHandlerActions(func(act codec.Action) error {
		prefix := s.regInfo.Prefix()
		key := strings.TrimPrefix(strings.Join([]string{prefix, s.regInfo.ServerInfo.Id, s.regInfo.Host, act.Id.String()}, "/"), "/")
		if reg {
			val := act.Name
			if s.flbNum > 0 {
				val = val + "|" + strconv.Itoa(s.flbNum)
			}
			if err := register.Register(s.app.Context(), key, val, s.regInfo.Ttl); err != nil {
				return err
			}
			s.logger.Debug(utils.ToStr("registered action:", key, "=>", val))
		} else {
			if err := register.Unregister(s.app.Context(), key); err != nil {
				return err
			}

			s.logger.Debug(utils.ToStr("unregistered action:", key))
		}
		return nil
	})
}

func (s *Handler) addErr(err error) {
	if err != nil {
		s.errs = append(s.errs, err)
	}
}

func (s *Handler) watch(register regCenter.Register) error {
	if register == nil {
		return nil
	}
	// watch tcp gateway
	tcpGwPrefix := s.tcpGwRegInfo.Prefix() + "/"
	err := register.Watch(s.app.Context(), tcpGwPrefix, func(key string, val string, isDel bool) {
		segments := strings.Split(key, "/")
		host := segments[len(segments)-1]
		if isDel {
			s.logger.Debug(utils.ToStr("tcp gateway [", host, "] leaved"))
			s.tcpGw.Manager().Rm("gateway", host)
		} else {
			s.logger.Debug(utils.ToStr("tcp gateway [", host, "] added"))
			s.tcpGw.Manager().Add("gateway", host)
		}
	})
	if err != nil {
		return err
	}
	wssGwPrefix := s.wssGwRegInfo.Prefix() + "/"
	return register.Watch(s.app.Context(), wssGwPrefix, func(key string, val string, isDel bool) {
		segments := strings.Split(key, "/")
		host := segments[len(segments)-1]
		if isDel {
			s.logger.Debug(utils.ToStr("wss gateway [", host, "] leaved"))
			s.wssGw.Manager().Rm("gateway", host)
		} else {
			s.logger.Debug(utils.ToStr("wss gateway [", host, "] added"))
			s.wssGw.Manager().Add("gateway", host)
		}
	})
}

func (s *Handler) initLogger() {
	s.logCnf = s.app.LogConfig()
	s.logger = s.app.Logger().Named(utils.ToStr(s.serverType.String(), "-", s.endType.String(), "-", s.id))
}

// SetWssActionFlbNum 用于解决 wss的action能负载到和tcp同一个服务上去, wss服务可用,(最好就是tcp的端口， 这样tcp负载算法输入和wss输入就一样了)
func (s *Handler) SetWssActionFlbNum(num int) {
	if s.socketType.ToServerType() == servertype.Wss && num > 0 {
		s.flbNum = num
	}
}
