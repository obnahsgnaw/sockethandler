package codec

import "sync"

type Name string

func (n Name) String() string {
	return string(n)
}

const (
	Proto Name = "proto"
	Json  Name = "json"
)

type DataBuilderProvider interface {
	Provider(Name) DataBuilder
}

type Dbp struct {
	json  DataBuilder
	proto DataBuilder
	sync.Once
}

func NewDbp() *Dbp {
	return &Dbp{}
}

func (p *Dbp) Provider(name Name) DataBuilder {
	if name == Json {
		p.Do(func() {
			p.json = NewJsonDataBuilder()
		})
		return p.json
	}

	p.Do(func() {
		p.proto = NewProtobufDataBuilder()
	})
	return p.proto
}
