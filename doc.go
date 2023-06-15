package sockethandler

import (
	"github.com/gin-gonic/gin"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/application/regtype"
	"github.com/obnahsgnaw/application/service/regCenter"
	"github.com/obnahsgnaw/sockethandler/sockettype"
	"net/http"
)

type DocConfig struct {
	id       string
	endType  endtype.EndType
	Origin   url.Origin
	RegTtl   int64
	CacheTtl int
	Doc      DocItem
}

type DocItem struct {
	Title      string
	socketType sockettype.SocketType // 通过上层应用来设置
	Path       string
	Provider   func() ([]byte, error)
}

type DocServer struct {
	config  *DocConfig
	engine  *gin.Engine
	regInfo *regCenter.RegInfo
}

// doc-index --> id-list --> key list

// NewDocServer new a socket doc server
func NewDocServer(clusterId string, config *DocConfig) *DocServer {
	gin.SetMode(gin.ReleaseMode)
	e := gin.Default()
	e.GET("/favicon.ico", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	s := &DocServer{
		config: config,
		engine: e,
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
		Host: config.Origin.Host.String(),
		Val:  "",
		Ttl:  config.RegTtl,
		Values: map[string]string{
			"title": config.Doc.Title,
			"url":   s.DocUrl(),
		},
	}

	return s
}

func (s *DocServer) RegInfo() *regCenter.RegInfo {
	return s.regInfo
}

func (s *DocServer) initDocRoute() {
	s.engine.GET(s.config.Doc.Path, func(c *gin.Context) {
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
	})
}

func (s *DocServer) Start() error {
	s.initDocRoute()
	if err := s.engine.Run(s.config.Origin.Host.String()); err != nil {
		return err
	}
	return nil
}

func (s *DocServer) DocUrl() string {
	return s.config.Origin.String() + s.config.Doc.Path
}

func (s *DocServer) SyncStart(cb func(error)) {
	go func() {
		if err := s.Start(); err != nil {
			cb(err)
		}
	}()
}
