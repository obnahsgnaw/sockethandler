package action

import (
	"context"
	"errors"
	"github.com/obnahsgnaw/sockethandler/service/codec"
	"sync"
)

type Manager struct {
	handlers            sync.Map // action-id, action-handler
	dateBuilderProvider codec.DataBuilderProvider
}

func NewManager() *Manager {
	return &Manager{
		dateBuilderProvider: codec.NewDbp(),
	}
}

type HandlerReq struct {
	ActionId uint32
	Gateway  string
	Fd       int64
	Id       string
	Type     string
	Format   string
	Package  []byte
}

type Handler func(context.Context, codec.DataBuilder, *HandlerReq) (codec.Action, []byte, error)

type actionHandler struct {
	Action  codec.Action
	Handler Handler
}

// RegisterHandler register an action with handler
func (m *Manager) RegisterHandler(action codec.Action, handler Handler) {
	m.handlers.Store(action.Id, actionHandler{Action: action, Handler: handler})
}

func (m *Manager) getHandler(actionId codec.ActionId) (Handler, bool) {
	if h, ok := m.handlers.Load(actionId); ok {
		return h.(actionHandler).Handler, true
	}

	return nil, false
}

func (m *Manager) GetAction(actionId codec.ActionId) (codec.Action, bool) {
	if h, ok := m.handlers.Load(actionId); ok {
		return h.(actionHandler).Action, true
	}

	return codec.Action{}, false
}

func (m *Manager) RangeHandlerActions(handle func(action codec.Action) error) (err error) {
	m.handlers.Range(func(key, value interface{}) bool {
		h := value.(actionHandler)
		if err = handle(h.Action); err != nil {
			return false
		}
		return true
	})
	return
}

// Dispatch the actions
func (m *Manager) Dispatch(ctx context.Context, q *HandlerReq) (respAction codec.Action, respData []byte, err error) {
	if actHandler, ok := m.handlers.Load(codec.ActionId(q.ActionId)); ok {
		respAction, respData, err = actHandler.(actionHandler).Handler(ctx, m.dateBuilderProvider.Provider(codec.Name(q.Format)), q)
		return
	}

	err = errors.New("NotFound")
	return
}
