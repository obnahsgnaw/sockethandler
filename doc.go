package sockethandler

import (
	"github.com/gin-gonic/gin"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/logging/logger"
	"github.com/obnahsgnaw/application/pkg/utils"
	"github.com/obnahsgnaw/application/regtype"
	"github.com/obnahsgnaw/application/servertype"
	"github.com/obnahsgnaw/application/service/regCenter"
	http2 "github.com/obnahsgnaw/http"
	"github.com/obnahsgnaw/http/engine"
	"github.com/obnahsgnaw/http/listener"
	"github.com/obnahsgnaw/sockethandler/sockettype"
	"net/http"
)

type DocConfig struct {
	id       string
	endType  endtype.EndType
	servType servertype.ServerType
	RegTtl   int64
	CacheTtl int
	Doc      DocItem
}

type DocItem struct {
	Title      string
	Public     bool
	socketType sockettype.SocketType // 通过上层应用来设置
	Module     string
	SubModule  string
	path       string
	Provider   func() ([]byte, error)
}

type DocServer struct {
	config  *DocConfig
	engine  *http2.Http
	regInfo *regCenter.RegInfo
	prefix  string
}

// doc-index --> id-list --> key list

func NewDocEngine(lr *listener.PortedListener, name string, cnf *logger.Config) (*http2.Http, error) {
	e, err := engine.New(&engine.Config{
		Name:           name,
		DebugMode:      false,
		LogDebug:       true,
		AccessWriter:   nil,
		ErrWriter:      nil,
		TrustedProxies: nil,
		Cors:           nil,
		LogCnf:         cnf,
	})
	if err != nil {
		return nil, err
	}
	return http2.New(e, lr), nil
}

func NewDocServer(e *http2.Http, clusterId string, config *DocConfig) *DocServer {
	s := &DocServer{
		config: config,
		engine: e,
		prefix: utils.ToStr("/", config.endType.String(), "-", config.Doc.socketType.ToServerType().String(), "-docs"), // the same prefix with the socket handler
	}
	s.config.Doc.path = utils.ToStr(s.prefix, "/", s.config.Doc.Module, "/", s.config.Doc.SubModule)
	public := "0"
	if config.Doc.Public {
		public = "1"
	}
	s.regInfo = &regCenter.RegInfo{
		AppId:   clusterId,
		RegType: regtype.Doc,
		ServerInfo: regCenter.ServerInfo{
			Id:      config.id,
			Name:    config.Doc.Title,
			Type:    config.Doc.socketType.String(),
			EndType: config.endType.String(),
		},
		Host: s.engine.Host().String(),
		Val:  "",
		Ttl:  config.RegTtl,
		Values: map[string]string{
			"title":  config.Doc.Title,
			"url":    s.DocUrl(),
			"public": public,
		},
	}
	s.initDocRoute()
	return s
}

func (s *DocServer) RegInfo() *regCenter.RegInfo {
	return s.regInfo
}

func (s *DocServer) initDocRoute() {
	hd := func(c *gin.Context) {
		if s.config.Doc.Provider == nil {
			c.Status(http.StatusOK)
			_, _ = c.Writer.Write([]byte("no doc provider"))
			return
		}
		tmpl, err := s.config.Doc.Provider()
		if err != nil {
			c.String(404, err.Error())
		} else {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.Status(http.StatusOK)
			_, _ = c.Writer.Write(tmpl)
		}
	}
	s.engine.Engine().GET(s.config.Doc.path, hd)
}

func (s *DocServer) Start() error {
	if err := s.engine.RunAndServ(); err != nil {
		return err
	}
	return nil
}

func (s *DocServer) DocUrl() string {
	return "http://" + s.engine.Host().String() + s.config.Doc.path
}

func (s *DocServer) SyncStart(cb func(error)) {
	go func() {
		if err := s.Start(); err != nil {
			cb(err)
		}
	}()
}
