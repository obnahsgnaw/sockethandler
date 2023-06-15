package codec

import "strconv"

type ActionId uint32

func (a ActionId) String() string {
	return strconv.Itoa(int(a))
}

// Action id
type Action struct {
	Id   ActionId
	Name string
}

func (a Action) String() string {
	return a.Id.String() + ":" + a.Name
}

func NewAction(id ActionId, name string) Action {
	return Action{
		Id:   id,
		Name: name,
	}
}
