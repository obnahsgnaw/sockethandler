package main

import (
	"context"
	"github.com/obnahsgnaw/application"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/debug"
	"github.com/obnahsgnaw/application/pkg/dynamic"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/sockethandler"
	"github.com/obnahsgnaw/sockethandler/service/action"
	"github.com/obnahsgnaw/sockethandler/sockettype"
	"github.com/obnahsgnaw/socketutil/codec"
	"log"
	"time"
)

func main() {
	app := application.New("dev", "dev", application.Debugger(debug.New(dynamic.NewBool(func() bool {
		return false
	}))))
	defer app.Release()
	app.With(application.EtcdRegister([]string{"127.0.0.1:2379"}, time.Second*5))

	s := sockethandler.New(
		app,
		"uav",
		"connect",
		"uav connect",
		endtype.Frontend,
		sockettype.TCP,
		url.Host{Ip: "127.0.0.1", Port: 8010},
	)

	s.WithDocServer(8011, "/v1/doc/socket/tcp", func() ([]byte, error) {
		return []byte("ok"), nil
	}, false)

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

	log.Println("Exited")
}
