package main

import (
	"context"
	"github.com/obnahsgnaw/application"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/logging/logger"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/application/service/regCenter"
	"github.com/obnahsgnaw/rpc"
	"github.com/obnahsgnaw/sockethandler"
	"github.com/obnahsgnaw/sockethandler/service/action"
	"github.com/obnahsgnaw/sockethandler/sockettype"
	"github.com/obnahsgnaw/socketutil/codec"
	"time"
)

func main() {
	reg, _ := regCenter.NewEtcdRegister([]string{"127.0.0.1:2379"}, time.Second*5)
	app := application.New(
		"demo",
		application.Debug(func() bool {
			return true
		}),
		application.Register(reg, 5),
		application.Logger(&logger.Config{
			Dir:        "/Users/wangshanbo/Documents/Data/projects/sockethandler/out",
			MaxSize:    5,
			MaxBackup:  1,
			MaxAge:     1,
			Level:      "debug",
			TraceLevel: "error",
		}),
	)
	defer app.Release()
	l, _ := rpc.NewListener(url.Host{Ip: "127.0.0.1", Port: 9011})
	r := sockethandler.NewRpc(app, "uav", "connect", endtype.Frontend, sockettype.TCP, l, nil)
	de, _ := sockethandler.NewDocEngine(l, "uav", nil)
	s := sockethandler.New(app, r,
		"uav",
		"connect",
		"uav connect",
		endtype.Frontend,
		sockettype.TCP,
		sockethandler.DocServ(
			de,
			func() ([]byte, error) {
				return []byte("ok"), nil
			},
			true,
		),
	)

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
