package impl

import (
	"context"
	"github.com/obnahsgnaw/rpc/pkg/rpcclient"
	bindv1 "github.com/obnahsgnaw/socketapi/gen/bind/v1"
	connv1 "github.com/obnahsgnaw/socketapi/gen/conninfo/v1"
	groupv1 "github.com/obnahsgnaw/socketapi/gen/group/v1"
	messagev1 "github.com/obnahsgnaw/socketapi/gen/message/v1"
	slbv1 "github.com/obnahsgnaw/socketapi/gen/slb/v1"
	"github.com/obnahsgnaw/socketutil/codec"
	"net"
	"sync"
	"time"
)

type Gateway struct {
	ctx context.Context
	m   *rpcclient.Manager
	dbp codec.DataBuilderProvider
}

func NewGateway(ctx context.Context, m *rpcclient.Manager) *Gateway {
	return &Gateway{
		ctx: ctx,
		m:   m,
		dbp: codec.NewDbp(),
	}
}

func (s *Gateway) Manager() *rpcclient.Manager {
	return s.m
}

func (s *Gateway) BindId(gw string, fd int64, id ...*bindv1.Id) error {
	cc, err := s.m.GetConn("gateway", gw, 0)
	if err != nil {
		return err
	}
	c := bindv1.NewBindServiceClient(cc)

	_, err = c.BindId(s.ctx, &bindv1.BindIdRequest{
		Fd:  fd,
		Ids: id,
	})
	return err
}

func (s *Gateway) UnBindId(gw string, fd int64, typ ...string) error {
	cc, err := s.m.GetConn("gateway", gw, 0)
	if err != nil {
		return err
	}
	c := bindv1.NewBindServiceClient(cc)

	_, err = c.UnBindId(s.ctx, &bindv1.UnBindIdRequest{
		Fd:    fd,
		Types: typ,
	})
	return err
}

func (s *Gateway) BindExist(gw string, id, typ string) (bool, error) {
	cc, err := s.m.GetConn("gateway", gw, 0)
	if err != nil {
		return false, err
	}
	c := bindv1.NewBindServiceClient(cc)

	p, err := c.BindExist(s.ctx, &bindv1.BindExistRequest{
		Id: &bindv1.Id{
			Typ: typ,
			Id:  id,
		},
	})
	if err != nil {
		return false, err
	}
	return p.Exist, nil
}

func (s *Gateway) BindExistAll(id, idType string) (bool, error) {
	for _, gw := range s.m.Get("gateway") {
		exist, err := s.BindExist(gw, id, idType)
		if err != nil {
			return false, err
		}
		if exist {
			return true, nil
		}
	}

	return false, nil
}

type ConnInfo struct {
	LocalAddr  net.Addr
	RemoteAddr net.Addr
	ConnectAt  time.Time
	SocketType string
	Uid        uint32
	UName      string
}
type Addr struct {
	net  string
	addr string
}

func (a Addr) Network() string {
	return a.net
}
func (a Addr) String() string {
	return a.addr
}

func (s *Gateway) ConnInfo(gw string, fd int64) (ConnInfo, error) {
	cc, err := s.m.GetConn("gateway", gw, 0)
	if err != nil {
		return ConnInfo{}, err
	}
	c := connv1.NewConnServiceClient(cc)

	resp, err := c.Info(s.ctx, &connv1.ConnInfoRequest{
		Fd: fd,
	})
	if err != nil {
		return ConnInfo{}, err
	}

	t, _ := time.Parse("2006-01-02 15:04:05", resp.ConnectAt)
	return ConnInfo{
		LocalAddr:  Addr{net: resp.LocalNetwork, addr: resp.LocalAddr},
		RemoteAddr: Addr{net: resp.RemoteNetwork, addr: resp.RemoteAddr},
		ConnectAt:  t,
		SocketType: resp.SocketType,
		Uid:        resp.Uid,
		UName:      resp.Uname,
	}, nil
}

func (s *Gateway) SendFdMessage(gw string, fd int64, act codec.Action, data codec.DataPtr) error {
	cc, err := s.m.GetConn("gateway", gw, 1)
	if err != nil {
		return err
	}
	c := messagev1.NewMessageServiceClient(cc)

	var pbMsg []byte
	var jsonMsg []byte
	if pbMsg, err = s.dbp.Provider(codec.Proto).Pack(data); err != nil {
		return err
	}
	if jsonMsg, err = s.dbp.Provider(codec.Json).Pack(data); err != nil {
		return err
	}

	_, err = c.SendMessage(s.ctx, &messagev1.SendMessageRequest{
		Target:      &messagev1.SendMessageRequest_Fd{Fd: fd},
		ActionId:    uint32(act.Id),
		ActionName:  act.Name,
		JsonMessage: jsonMsg,
		PbMessage:   pbMsg,
	})
	return err
}

func (s *Gateway) SendIdMessage(gw string, id *messagev1.SendMessageRequest_BindId, act codec.Action, data codec.DataPtr) error {
	cc, err := s.m.GetConn("gateway", gw, 1)
	if err != nil {
		return err
	}
	c := messagev1.NewMessageServiceClient(cc)

	var pbMsg []byte
	var jsonMsg []byte
	if pbMsg, err = s.dbp.Provider(codec.Proto).Pack(data); err != nil {
		return err
	}
	if jsonMsg, err = s.dbp.Provider(codec.Json).Pack(data); err != nil {
		return err
	}

	_, err = c.SendMessage(s.ctx, &messagev1.SendMessageRequest{
		Target: &messagev1.SendMessageRequest_Id{
			Id: id,
		},
		ActionId:    uint32(act.Id),
		ActionName:  act.Name,
		JsonMessage: jsonMsg,
		PbMessage:   pbMsg,
	})
	return err
}

func (s *Gateway) SendIdMessageAll(id *messagev1.SendMessageRequest_BindId, act codec.Action, data codec.DataPtr) (err error) {
	for _, gw := range s.m.Get("gateway") {
		err = s.SendIdMessage(gw, id, act, data)
		if err == nil {
			return
		}
	}

	return
}

func (s *Gateway) JoinGroup(gw string, group, id string, fd int64) error {
	cc, err := s.m.GetConn("gateway", gw, 2)
	if err != nil {
		return err
	}
	c := groupv1.NewGroupServiceClient(cc)

	_, err = c.JoinGroup(s.ctx, &groupv1.JoinGroupRequest{
		Group: &groupv1.Group{Name: group},
		Member: &groupv1.Member{
			Fd: fd,
			Id: id,
		},
	})
	return err
}

func (s *Gateway) LeaveGroup(gw string, group string, fd int64) error {
	cc, err := s.m.GetConn("gateway", gw, 2)
	if err != nil {
		return err
	}
	c := groupv1.NewGroupServiceClient(cc)

	_, err = c.LeaveGroup(s.ctx, &groupv1.LeaveGroupRequest{
		Group: &groupv1.Group{Name: group},
		Fd:    fd,
	})
	return err
}

func (s *Gateway) Broadcast(gw string, group string, act codec.Action, data codec.DataPtr, id string) error {
	cc, err := s.m.GetConn("gateway", gw, 2)
	if err != nil {
		return err
	}
	c := groupv1.NewGroupServiceClient(cc)

	var pbMsg []byte
	var jsonMsg []byte
	if pbMsg, err = s.dbp.Provider(codec.Proto).Pack(data); err != nil {
		return err
	}
	if jsonMsg, err = s.dbp.Provider(codec.Json).Pack(data); err != nil {
		return err
	}

	_, err = c.BroadcastGroup(s.ctx, &groupv1.BroadcastGroupRequest{
		Group:       &groupv1.Group{Name: group},
		ActionId:    uint32(act.Id),
		ActionName:  act.Name,
		JsonMessage: jsonMsg,
		PbMessage:   pbMsg,
		Id:          id,
	})
	return err
}

func (s *Gateway) BroadcastAll(group string, act codec.Action, data codec.DataPtr, id string) {
	var wg sync.WaitGroup
	for _, gw := range s.m.Get("gateway") {
		wg.Add(1)
		go func(gw1 string) {
			_ = s.Broadcast(gw1, group, act, data, id)
			wg.Done()
		}(gw)
	}
	wg.Wait()
}

func (s *Gateway) SetActionSlb(gw string, fd, action, slb int64) error {
	cc, err := s.m.GetConn("gateway", gw, 0)
	if err != nil {
		return err
	}
	c := slbv1.NewSlbServiceClient(cc)

	_, err = c.SetActionSlb(s.ctx, &slbv1.ActionSlbRequest{
		Fd:     fd,
		Action: action,
		Sbl:    slb,
	})
	return err
}
