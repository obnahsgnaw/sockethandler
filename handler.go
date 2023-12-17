package sockethandler

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
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
	handlerv1 "github.com/obnahsgnaw/socketapi/gen/handler/v1"
	"github.com/obnahsgnaw/sockethandler/service/action"
	"github.com/obnahsgnaw/sockethandler/service/proto/impl"
	"github.com/obnahsgnaw/sockethandler/sockettype"
	"github.com/obnahsgnaw/socketutil/codec"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"path/filepath"
	"strconv"
	"strings"
)

type Handler struct {
	id           string
	module       string
	subModule    string
	name         string
	st           servertype.ServerType
	sct          sockettype.SocketType
	et           endtype.EndType
	host         url.Host
	app          *application.Application
	am           *action.Manager
	tpm          *rpc.Manager
	wpm          *rpc.Manager
	rs           *rpc2.Server
	rsCus        bool
	logger       *zap.Logger
	ds           *DocServer
	dsCus        bool
	dsPrefixed   bool
	regInfo      *regCenter.RegInfo // actions
	tcpGwRegInfo *regCenter.RegInfo // tcp gateway
	wssGwRegInfo *regCenter.RegInfo // wss gateway
	tgw          *impl.Gateway
	wgw          *impl.Gateway
	errs         []error
	flbNum       int
	actListeners []func(manager *action.Manager)
}

func New(app *application.Application, module, subModule, name string, et endtype.EndType, sct sockettype.SocketType) *Handler {
	var err error
	s := &Handler{
		id:        module + "-" + subModule,
		module:    module,
		subModule: subModule,
		name:      name,
		st:        sct.ToHandlerType(),
		sct:       sct,
		et:        et,
		app:       app,
		am:        action.NewManager(),
		tpm:       rpc.NewManager(),
		wpm:       rpc.NewManager(),
	}
	s.tgw = impl.NewGateway(app.Context(), s.tpm)
	s.wgw = impl.NewGateway(app.Context(), s.wpm)
	if s.st == "" {
		s.addErr(s.handlerError("type invalid", errors.New("type not support")))
	}
	logCnf := logger.CopyCnfWithLevel(s.app.LogConfig())
	if logCnf != nil {
		logCnf.AddSubDir(filepath.Join(s.et.String(), utils.ToStr(s.st.String(), "-", s.id)))
		logCnf.SetFilename(utils.ToStr(s.st.String(), "-", s.id))
		logCnf.ReplaceTraceLevel(zap.NewAtomicLevelAt(zap.FatalLevel))
	}
	s.logger, err = logger.New(utils.ToStr(s.st.String(), ":", s.id), logCnf, s.app.Debugger().Debug())
	s.addErr(err)
	s.tpm.RegisterAfterHandler(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, err error, opts ...grpc.CallOption) {
		if err != nil {
			s.logger.Error(utils.ToStr("rpc call tcp-gateway[", method, "] failed, ", err.Error()), zap.Any("req", req), zap.Any("resp", reply))
		} else {
			s.logger.Debug(utils.ToStr("rpc call tcp-gateway[", method, "] success"), zap.Any("req", req), zap.Any("resp", reply))
		}
	})
	s.wpm.RegisterAfterHandler(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, err error, opts ...grpc.CallOption) {
		if err != nil {
			s.logger.Error(utils.ToStr("rpc call wss-gateway[", method, "] failed, ", err.Error()), zap.Any("req", req), zap.Any("resp", reply))
		} else {
			s.logger.Debug(utils.ToStr("rpc call wss-gateway[", method, "] success"), zap.Any("req", req), zap.Any("resp", reply))
		}
	})
	s.tcpGwRegInfo = &regCenter.RegInfo{
		AppId:   app.ID(),
		RegType: regtype.Rpc,
		ServerInfo: regCenter.ServerInfo{
			Id:      sockettype.TCP.String() + "-gateway",
			Name:    "",
			Type:    servertype.Tcp.String(),
			EndType: s.et.String(),
		},
	}
	s.wssGwRegInfo = &regCenter.RegInfo{
		AppId:   app.ID(),
		RegType: regtype.Rpc,
		ServerInfo: regCenter.ServerInfo{
			Id:      sockettype.WSS.String() + "-gateway",
			Name:    "",
			Type:    servertype.Wss.String(),
			EndType: s.et.String(),
		},
	}

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
func (s *Handler) WithDocServer(port int, docProxyPrefix string, provider func() ([]byte, error), public bool, projPrefixed bool) {
	s.dsPrefixed = projPrefixed
	s.ds = NewDocServer(s.app.ID(), s.docConfig(port, docProxyPrefix, provider, public))
}

// WithDocServerIns with doc server
func (s *Handler) WithDocServerIns(ins *gin.Engine, port int, docProxyPrefix string, provider func() ([]byte, error), public bool) {
	s.dsCus = true
	s.ds = NewDocServerWithEngine(ins, s.app.ID(), s.docConfig(port, docProxyPrefix, provider, public))
}

func (s *Handler) docConfig(port int, docProxyPrefix string, provider func() ([]byte, error), public bool) *DocConfig {
	if docProxyPrefix != "" {
		docProxyPrefix = "/" + strings.Trim(docProxyPrefix, "/")
	}
	docName := "docs"
	if s.dsCus || s.dsPrefixed {
		docName = utils.ToStr(s.et.String(), "-", s.sct.ToServerType().String(), "-docs")
	}
	return &DocConfig{
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
			Path:       utils.ToStr("/", docName, "/", s.module, "/", s.subModule, ".", s.sct.ToServerType().String()+"doc"), // the same with the socket gateway,
			Prefix:     docProxyPrefix,
			Title:      s.name,
			Public:     public,
			Provider:   provider,
		},
	}
}

func (s *Handler) WithRpc(host url.Host) {
	s.host = host
	s.initRegInfo()
}

func (s *Handler) initRegInfo() {
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
}

func (s *Handler) initRs(failedCb func(error)) bool {
	if s.rs == nil {
		if s.host.Port <= 0 {
			failedCb(s.handlerError("port err", errors.New("handler rpc port required")))
			return false
		}
		s.rs = rpc2.New(s.app, s.id, utils.ToStr(s.st.String(), "-", s.id, "-rpc"), s.et, s.host, rpc2.Parent(s))
		s.logger.Debug("rpc initialized(default)")
	} else {
		s.logger.Debug("rpc initialized(customer)")
	}
	s.rs.RegisterService(rpc2.ServiceInfo{
		Desc: handlerv1.HandlerService_ServiceDesc,
		Impl: impl.NewHandlerService(s.am),
	})
	return true
}

func (s *Handler) WithRpcIns(ins *rpc2.Server) {
	s.rs = ins
	s.rsCus = true
	s.host = ins.Host()
	s.initRegInfo()
	// 不注册， 这里注册action
}

// Release resource
func (s *Handler) Release() {
	if s.rs != nil {
		s.rs.Release()
	}
	if s.app.Register() != nil {
		if s.ds != nil {
			_ = s.app.DoUnregister(s.ds.RegInfo(), func(msg string) {
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

// Run start run
func (s *Handler) Run(failedCb func(error)) {
	if s.errs != nil {
		failedCb(s.errs[0])
		return
	}
	s.logger.Info("init start...")
	if !s.initRs(failedCb) {
		return
	}
	if s.ds != nil {
		s.logger.Info(utils.ToStr("doc server[", s.ds.config.Origin.Host.String(), "] start and serving..."))
		s.logger.Info("doc url=" + s.ds.DocUrl())
		s.ds.SyncStart(failedCb)
		if s.app.Register() != nil {
			if err := s.app.DoRegister(s.ds.RegInfo(), func(msg string) {
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
		h(s.am)
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
	if !s.rsCus {
		s.logger.Info(utils.ToStr("server[", s.host.String(), "] start and serving..."))
		s.rs.Run(failedCb)
	}
}

func (s *Handler) handlerError(msg string, err error) error {
	return utils.TitledError(utils.ToStr("handler[", s.name, "] error"), msg, err)
}

// Listen action
func (s *Handler) Listen(act codec.Action, structure action.DataStructure, handler action.Handler) {
	s.actListeners = append(s.actListeners, func(manager *action.Manager) {
		manager.RegisterHandler(act, structure, handler)
		s.logger.Debug("listened action:" + act.Name)
	})
}

func (s *Handler) register(register regCenter.Register, reg bool) error {
	return s.am.RangeHandlerActions(func(act codec.Action) error {
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
			s.tpm.Rm("gateway", host)
		} else {
			s.logger.Debug(utils.ToStr("tcp gateway [", host, "] added"))
			s.tpm.Add("gateway", host)
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
			s.wpm.Rm("gateway", host)
		} else {
			s.logger.Debug(utils.ToStr("wss gateway [", host, "] added"))
			s.wpm.Add("gateway", host)
		}
	})
}

func (s *Handler) TcpGateway() *impl.Gateway {
	return s.tgw
}

func (s *Handler) WssGateway() *impl.Gateway {
	return s.wgw
}

// SetWssActionFlbNum 用于解决 wss的action能负载到和tcp同一个服务上去, wss服务可用,(最好就是tcp的端口， 这样tcp负载算法输入和wss输入就一样了)
func (s *Handler) SetWssActionFlbNum(num int) {
	if s.sct.ToServerType() == servertype.Wss && num > 0 {
		s.flbNum = num
	}
}
