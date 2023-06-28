package main

import (
	"github.com/obnahsgnaw/application"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/debug"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/sockethandler"
	"github.com/obnahsgnaw/sockethandler/sockettype"
	"log"
	"time"
)

func main() {
	app := application.New("demo", "demo", application.Debugger(debug.New(true)))
	app.With(application.EtcdRegister([]string{"127.0.0.1:2379"}, time.Second*5))

	s := sockethandler.New(
		app,
		"uav",
		"connect",
		"uav connect",
		endtype.Backend,
		sockettype.TCP,
		url.Host{Ip: "127.0.0.1", Port: 8010},
	)

	s.WithDocServer(8011, "/tcp", func() ([]byte, error) {
		return []byte("ok"), nil
	})

	app.AddServer(s)

	app.Run(func(err error) {
		panic(err)
	})

	log.Println("Exited")
}
