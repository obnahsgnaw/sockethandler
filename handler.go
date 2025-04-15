package sockethandler

import (
	"context"
	"github.com/obnahsgnaw/application"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/logging/logger"
	"github.com/obnahsgnaw/application/pkg/security"
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
	app             *application.Application
	id              string
	module          string
	subModule       string
	name            string
	endType         endtype.EndType
	businessChannel string
	rpcServer       *ManagedRpc
	logger          *zap.Logger
	logCnf          *logger.Config
	engin           *http.Http
	docServer       *DocServer
	regInfo         *regCenter.RegInfo            // actions
	gateway         *impl.Gateway                 // current channel  gateway
	gateways        map[string]*impl.Gateway      // businessChannel => gateway
	watchGwRegInfos map[string]*regCenter.RegInfo // businessChannel => gateway
	errs            []error
	flbNum          int
	actListeners    []func(manager *action.Manager)
	running         bool
}

func New(app *application.Application, rps *ManagedRpc, module, subModule, name string, et endtype.EndType, businessChannel string, o ...Option) *Handler {
	s := &Handler{
		id:              module + "-" + subModule,
		module:          module,
		subModule:       subModule,
		name:            name,
		app:             app,
		endType:         et,
		businessChannel: businessChannel,
		rpcServer:       rps,
		gateways:        make(map[string]*impl.Gateway),
		watchGwRegInfos: make(map[string]*regCenter.RegInfo),
	}
	s.initLogger()
	s.initRegInfo()
	gw, reg := s.initChannelGateway(businessChannel)
	s.gateway = gw
	s.gateways[businessChannel] = gw
	s.watchGwRegInfos[businessChannel] = reg

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
	return "handler"
}

// EndType return the server end type
func (s *Handler) EndType() endtype.EndType {
	return s.endType
}

// BusinessChannel return the server business channel
func (s *Handler) BusinessChannel() string {
	return s.businessChannel
}

// Run start run
func (s *Handler) Run(failedCb func(error)) {
	if s.running {
		return
	}
	if s.errs != nil {
		failedCb(s.errs[0])
		return
	}
	s.logger.Info("init start...")
	if s.docServer != nil {
		s.logger.Debug("doc server enabled")
		s.logger.Info("doc url=" + s.docServer.DocUrl())
		s.logger.Info(utils.ToStr("doc server[", s.docServer.engine.Host(), "] start and serving..."))
		s.docServer.SyncStart(security.RandAlpha(6), failedCb)
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
		h(s.rpcServer.Manager().GetManager(s.businessChannel))
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
	s.logger.Info(utils.ToStr("server[", s.rpcServer.Server().Host().String(), "] start and serving..."))
	s.rpcServer.Server().Run(failedCb)
	s.running = true
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
	s.running = false
}

func (s *Handler) Gateway() *impl.Gateway {
	return s.gateway
}

func (s *Handler) TcpGateway() *impl.Gateway {
	return s.ChannelGateway("outer")
}

func (s *Handler) WssGateway() *impl.Gateway {
	return s.ChannelGateway("inner")
}

func (s *Handler) ChannelGateway(channel string) *impl.Gateway {
	if v, ok := s.gateways[channel]; ok {
		return v
	}
	gw, regInfo := s.initChannelGateway(channel)
	s.gateways[channel] = gw
	s.watchGwRegInfos[channel] = regInfo
	_ = s.watchGw(s.app.Register(), gw, regInfo)
	return gw
}

func (s *Handler) initChannelGateway(channel string) (*impl.Gateway, *regCenter.RegInfo) {
	gw := impl.NewGateway(s.app.Context(), channel+"-"+s.module+"-"+s.subModule, rpcclient.NewManager())
	gw.Manager().RegisterAfterHandler(func(ctx context.Context, head rpcclient.Header, method string, req, reply interface{}, cc *grpc.ClientConn, err error, opts ...grpc.CallOption) {
		if err != nil {
			s.logger.Error(utils.ToStr(head.RqId, " ", head.From, " rpc call ", head.To, " ", channel, "-gateway[", method, "] failed,", err.Error()), zap.Any("rq_id", head.RqId), zap.Any("req", req), zap.Any("resp", reply))
		} else {
			s.logger.Debug(utils.ToStr(head.RqId, " ", head.From, " rpc call ", head.To, " ", channel, "-gateway[", method, "] success"), zap.Any("rq_id", head.RqId), zap.Any("req", req), zap.Any("resp", reply))
		}
	})
	regInfo := &regCenter.RegInfo{
		AppId:   s.app.Cluster().Id(),
		RegType: regtype.Rpc,
		ServerInfo: regCenter.ServerInfo{
			Type:    "socket-gw@" + channel,
			EndType: s.endType.String(),
		},
	}
	return gw, regInfo
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
			if act.Id != 0 {
				panic("action[" + act.String() + "] already listened.")
			}
		}
		manager.RegisterHandler(act, structure, handler)
		s.logger.Debug("listened action:" + act.Name)
	})
}

func (s *Handler) docConfig(provider func() ([]byte, error), public bool) *DocConfig {
	return &DocConfig{
		id:       s.id,
		servType: servertype.ServerType(s.businessChannel),
		endType:  s.endType,
		RegTtl:   s.app.RegTtl(),
		Doc: DocItem{
			socketType: sockettype.SocketType(s.businessChannel),
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
			Type:    "socket-hdl@" + s.businessChannel,
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
	return s.rpcServer.Manager().GetManager(s.businessChannel).RangeHandlerActions(func(act codec.Action) error {
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
	for ch, gw := range s.gateways {
		if err := s.watchGw(register, gw, s.watchGwRegInfos[ch]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Handler) watchGw(register regCenter.Register, gw *impl.Gateway, regInfo *regCenter.RegInfo) error {
	prefix := regInfo.Prefix() + "/"
	return register.Watch(s.app.Context(), prefix, func(key string, val string, isDel bool) {
		segments := strings.Split(key, "/")
		host := segments[len(segments)-1]
		if isDel {
			s.logger.Debug(utils.ToStr(gw.Id()+": gateway [", host, "] leaved"))
			gw.Manager().Rm("gateway", host)
		} else {
			s.logger.Debug(utils.ToStr(gw.Id()+": gateway [", host, "] added"))
			gw.Manager().Add("gateway", host)
		}
	})
}

func (s *Handler) initLogger() {
	s.logCnf = s.app.LogConfig()
	s.logger = s.app.Logger().Named(utils.ToStr(s.businessChannel, "-", s.id, s.endType.String(), "-handler"))
}

// SetActionFlbNum 用于解决 action能负载到和同一个服务上去, 内部业务与外部业务使用相同的服务,(最好就是主服务的端口，都和主服务一样的轮训)
func (s *Handler) SetActionFlbNum(num int) {
	if num > 0 {
		s.flbNum = num
	}
}

func (s *Handler) SetWssActionFlbNum(num int) {
	if s.businessChannel == "inner" && num > 0 {
		s.flbNum = num
	}
}
