package main

import (
	"github.com/obnahsgnaw/application"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/sockethandler"
	"github.com/obnahsgnaw/sockethandler/sockettype"
	"log"
)

func main() {
	app := application.New("demo", "Demo")

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
