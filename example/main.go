package main

import (
	"context"
	"github.com/obnahsgnaw/application"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/logging/logger"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/sockethandler"
	"github.com/obnahsgnaw/sockethandler/service/action"
	"github.com/obnahsgnaw/sockethandler/sockettype"
	"github.com/obnahsgnaw/socketutil/codec"
	"time"
)

func main() {
	app := application.New(application.NewCluster("dev", "Dev"), "demo")
	defer app.Release()
	app.With(application.Debug(func() bool {
		return true
	}))
	app.With(application.EtcdRegister([]string{"127.0.0.1:2379"}, time.Second*5))
	app.With(application.Logger(&logger.Config{
		Dir:        "/Users/wangshanbo/Documents/Data/projects/sockethandler/out",
		MaxSize:    5,
		MaxBackup:  1,
		MaxAge:     1,
		Level:      "debug",
		TraceLevel: "error",
	}))

	s := sockethandler.New(
		app,
		"uav",
		"connect",
		"uav connect",
		endtype.Frontend,
		sockettype.TCP,
	)
	s.WithRpc(url.Host{Ip: "127.0.0.1", Port: 8010})

	s.WithDocServer(8011, "/v1/doc/socket/tcp", func() ([]byte, error) {
		return []byte("ok"), nil
	}, false, false)

	s.Listen(codec.Action{
		Id:   101,
		Name: "test",
	}, func() codec.DataPtr {
		return nil
	}, func(ctx context.Context, req *action.HandlerReq) (codec.Action, codec.DataPtr, error) {
		return codec.Action{
			Id:   102,
			Name: "test",
		}, nil, nil
	})

	app.AddServer(s)

	app.Run(func(err error) {
		panic(err)
	})

	app.Wait()
}
