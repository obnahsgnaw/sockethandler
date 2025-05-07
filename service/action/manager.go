package action

import (
	"context"
	"github.com/obnahsgnaw/socketutil/codec"
	"strconv"
	"strings"
	"sync"
)

type Manager struct {
	handlers       sync.Map // action-id, action-handler
	moduleHandlers sync.Map // module@action-id, action-handler
	closeHandlers  []Handler
	closeAction    codec.Action
}

func NewManager() *Manager {
	return &Manager{}
}

type HandlerReq struct {
	Action  codec.Action
	Gateway string
	Fd      int64
	idMap   map[string]string
	Data    codec.DataPtr
	User    *User
	Target  *Target
}

func (q *HandlerReq) BondId(typ string) (string, bool) {
	id, ok := q.idMap[typ]
	return id, ok
}

func NewHandlerReq(gw string, action codec.Action, fd int64, u *User, data codec.DataPtr, ids map[string]string, target *Target) *HandlerReq {
	if target == nil {
		target = &Target{}
	}
	return &HandlerReq{
		Action:  action,
		Gateway: gw,
		Fd:      fd,
		idMap:   ids,
		Data:    data,
		User:    u,
		Target:  target,
	}
}

type User struct {
	Id   uint32
	Name string
	Attr map[string]string
}

func (s *User) GetAttr(key, defVal string) string {
	if v, ok := s.Attr[key]; ok {
		return v
	}
	return defVal
}

func (s *User) Uid() uint32 {
	return s.Id
}

func (s *User) Uno() string {
	return s.GetAttr("user_id", "")
}

func (s *User) Cid() uint32 {
	v, _ := strconv.Atoi(s.GetAttr("company_id", "0"))
	return uint32(v)
}

func (s *User) Cno() string {
	return s.GetAttr("company_no", "")
}

func (s *User) CProject() string {
	return s.GetAttr("company_project", "")
}

func (s *User) COid() uint32 {
	v, _ := strconv.Atoi(s.GetAttr("company_organization_id", "0"))
	return uint32(v)
}

func (s *User) Oid() uint32 {
	v, _ := strconv.Atoi(s.GetAttr("organization_id", "0"))
	return uint32(v)
}

type Target struct {
	Type      string
	Id        string
	Iid       uint32
	Sn        string
	Cid       uint32
	Uid       uint32
	Protocol  string
	SessionId string
}

type Handler func(context.Context, *HandlerReq) (codec.Action, codec.DataPtr, error)

type DataStructure func() codec.DataPtr

type actionHandler struct {
	action    codec.Action
	structure DataStructure
	handler   Handler
}

// RegisterHandler register an action with handler
func (m *Manager) RegisterHandler(module string, action codec.Action, ds DataStructure, handler Handler) {
	if action.Id == m.closeAction.Id {
		m.registerCloseHandler(handler)
		return
	}
	m.handlers.Store(action.Id, actionHandler{action: action, structure: ds, handler: handler})
	m.moduleHandlers.Store(module+"@"+strconv.Itoa(int(action.Id)), action)
}

func (m *Manager) registerCloseHandler(handler Handler) {
	m.closeHandlers = append(m.closeHandlers, handler)
	if _, ok := m.handlers.Load(m.closeAction.Id); !ok {
		m.handlers.Store(m.closeAction.Id, actionHandler{action: m.closeAction, structure: func() codec.DataPtr { return nil }, handler: func(ctx context.Context, req *HandlerReq) (codec.Action, codec.DataPtr, error) {
			for _, h := range m.closeHandlers {
				_, _, _ = h(ctx, req)
			}
			return codec.Action{}, nil, nil
		}})
	}
}

func (m *Manager) GetHandler(actionId codec.ActionId) (codec.Action, DataStructure, Handler, bool) {
	if h, ok := m.handlers.Load(actionId); ok {
		h1 := h.(actionHandler)
		return h1.action, h1.structure, h1.handler, true
	}

	return codec.Action{}, nil, nil, false
}

func (m *Manager) RangeHandlerActions(module string, handle func(action codec.Action) error) (err error) {
	m.moduleHandlers.Range(func(key, value interface{}) bool {
		keyStr := key.(string)
		if strings.HasPrefix(keyStr, module+"@") {
			if err = handle(value.(codec.Action)); err != nil {
				return false
			}
		}
		return true
	})
	return
}
