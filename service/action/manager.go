package action

import (
	"context"
	"github.com/obnahsgnaw/sockethandler/service/codec"
	"sync"
)

type Manager struct {
	handlers sync.Map // action-id, action-handler
}

func NewManager() *Manager {
	return &Manager{}
}

type HandlerReq struct {
	Action  codec.Action
	Gateway string
	Fd      int64
	Id      string
	Type    string
	Data    codec.DataPtr
	User    *User
}

type User struct {
	Id   uint32
	Name string
}

type Handler func(context.Context, *HandlerReq) (codec.Action, codec.DataPtr, error)

type DataStructure func() codec.DataPtr

type actionHandler struct {
	action    codec.Action
	structure DataStructure
	handler   Handler
}

// RegisterHandler register an action with handler
func (m *Manager) RegisterHandler(action codec.Action, ds DataStructure, handler Handler) {
	m.handlers.Store(action.Id, actionHandler{action: action, structure: ds, handler: handler})
}

func (m *Manager) GetHandler(actionId codec.ActionId) (codec.Action, DataStructure, Handler, bool) {
	if h, ok := m.handlers.Load(actionId); ok {
		h1 := h.(actionHandler)
		return h1.action, h1.structure, h1.handler, true
	}

	return codec.Action{}, nil, nil, false
}

func (m *Manager) RangeHandlerActions(handle func(action codec.Action) error) (err error) {
	m.handlers.Range(func(key, value interface{}) bool {
		h := value.(actionHandler)
		if err = handle(h.action); err != nil {
			return false
		}
		return true
	})
	return
}
