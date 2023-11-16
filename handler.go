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
	handlerv1 "github.com/obnahsgnaw/socketapi/gen/handler/v1"
	"github.com/obnahsgnaw/sockethandler/service/action"
	"github.com/obnahsgnaw/sockethandler/service/proto/impl"
	"github.com/obnahsgnaw/sockethandler/sockettype"
	"github.com/obnahsgnaw/socketutil/codec"
	"go.uber.org/zap"
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
	logger       *zap.Logger
	ds           *DocServer
	regInfo      *regCenter.RegInfo // actions
	tcpGwRegInfo *regCenter.RegInfo // tcp gateway
	wssGwRegInfo *regCenter.RegInfo // wss gateway
	tgw          *impl.Gateway
	wgw          *impl.Gateway
	errs         []error
	flbNum       int
}

func New(app *application.Application, module, subModule, name string, et endtype.EndType, sct sockettype.SocketType, rpcHost url.Host) *Handler {
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
		host:      rpcHost,
	}
	s.tgw = impl.NewGateway(app.Context(), s.tpm)
	s.wgw = impl.NewGateway(app.Context(), s.wpm)
	if s.st == "" {
		s.addErr(errors.New(s.msg("type not support")))
	}
	s.logger, err = logger.New(utils.ToStr("Hdl[", s.et.String(), "][", s.sct.String(), "-", module+"-"+subModule, "]"), s.app.LogConfig(), s.app.Debugger().Debug())
	s.addErr(err)
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
func (s *Handler) WithDocServer(port int, docProxyPrefix string, provider func() ([]byte, error), public bool) {
	if docProxyPrefix != "" {
		docProxyPrefix = "/" + strings.Trim(docProxyPrefix, "/")
	}
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
			Path:       "/docs/" + s.module + "/" + s.subModule + "." + s.sct.ToServerType().String() + "doc", // the same with the socket gateway
			Prefix:     docProxyPrefix,
			Title:      s.name,
			Public:     public,
			Provider:   provider,
		},
	}
	s.ds = NewDocServer(s.app.ID(), config)
	s.debug("doc server enabled")
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
	s.debug("released")
}

// Run start run
func (s *Handler) Run(failedCb func(error)) {
	if s.errs != nil {
		failedCb(s.errs[0])
		return
	}
	s.debug("start running...")
	if s.app.Register() != nil {
		if err := s.register(s.app.Register(), true); err != nil {
			failedCb(s.handlerError(s.msg("register failed"), err))
		}
	}
	if s.ds != nil {
		docDesc := utils.ToStr("doc server [", s.ds.config.Origin.Host.String(), "] ")
		s.logger.Info(docDesc + "start and serving...")
		s.logger.Info(docDesc + "socket doc url=" + s.ds.DocUrl())
		s.ds.SyncStart(failedCb)
	}
	if err := s.watch(s.app.Register()); err != nil {
		failedCb(err)
		return
	}
	if s.rs != nil {
		s.rs.Run(failedCb)
	}
}

func (s *Handler) handlerError(msg string, err error) error {
	return utils.TitledError(utils.ToStr("handler[", s.name, "] error"), msg, err)
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
			val := act.Name
			if s.flbNum > 0 {
				val = val + "|" + strconv.Itoa(s.flbNum)
			}
			if err := register.Register(s.app.Context(), key, val, s.regInfo.Ttl); err != nil {
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

func (s *Handler) addErr(err error) {
	if err != nil {
		s.errs = append(s.errs, err)
	}
}

func (s *Handler) msg(msg ...string) string {
	return utils.ToStr("Socket Handler[", s.name, "] ", utils.ToStr(msg...))
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
			s.debug(utils.ToStr("tcp gateway [", host, "] leaved"))
			s.tpm.Rm("gateway", host)
		} else {
			s.debug(utils.ToStr("tcp gateway [", host, "] added"))
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
			s.debug(utils.ToStr("wss gateway [", host, "] leaved"))
			s.wpm.Rm("gateway", host)
		} else {
			s.debug(utils.ToStr("wss gateway [", host, "] added"))
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
