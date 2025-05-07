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
	"google.golang.org/grpc"
	"net"
	"strings"
	"sync"
	"time"
)

type Gateway struct {
	ctx context.Context
	m   *rpcclient.Manager
	dbp codec.DataBuilderProvider
	id  string
}

func NewGateway(ctx context.Context, id string, m *rpcclient.Manager) *Gateway {
	return &Gateway{
		ctx: ctx,
		m:   m,
		dbp: codec.NewDbp(),
		id:  id,
	}
}

func (s *Gateway) Id() string {
	return s.id
}

func (s *Gateway) Manager() *rpcclient.Manager {
	return s.m
}

func (s *Gateway) ParseRqId(gw string) (string, string) {
	if strings.Contains(gw, ":@") {
		gws := strings.Split(gw, "@")
		return gws[1], gws[0]
	}
	return gw, ""
}

func (s *Gateway) BindId(gw string, fd int64, id ...*bindv1.Id) error {
	var rqId string
	gw, rqId = s.ParseRqId(gw)
	return s.m.HostCall(s.ctx, gw, 0, s.id, "gateway", rqId, "", "", func(ctx context.Context, cc *grpc.ClientConn) error {
		c := bindv1.NewBindServiceClient(cc)

		_, err := c.BindId(s.ctx, &bindv1.BindIdRequest{
			Fd:  fd,
			Ids: id,
		})
		return err
	})
}

func (s *Gateway) UnBindId(gw string, fd int64, typ ...string) error {
	var rqId string
	gw, rqId = s.ParseRqId(gw)
	return s.m.HostCall(s.ctx, gw, 0, s.id, "gateway", rqId, "", "", func(ctx context.Context, cc *grpc.ClientConn) error {
		c := bindv1.NewBindServiceClient(cc)

		_, err := c.UnBindId(s.ctx, &bindv1.UnBindIdRequest{
			Fd:    fd,
			Types: typ,
		})
		return err
	})
}

func (s *Gateway) BindExist(gw string, id, typ string) (bool, error) {
	var rqId string
	gw, rqId = s.ParseRqId(gw)
	var p *bindv1.BindExistResponse
	err := s.m.HostCall(s.ctx, gw, 0, s.id, "gateway", rqId, "", "", func(ctx context.Context, cc *grpc.ClientConn) error {
		c := bindv1.NewBindServiceClient(cc)

		var err1 error
		p, err1 = c.BindExist(s.ctx, &bindv1.BindExistRequest{
			Id: &bindv1.Id{
				Typ: typ,
				Id:  id,
			},
		})
		return err1
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

func (s *Gateway) BindProxyTarget(gw string, fd int64, target ...string) error {
	var rqId string
	gw, rqId = s.ParseRqId(gw)
	return s.m.HostCall(s.ctx, gw, 0, s.id, "gateway", rqId, "", "", func(ctx context.Context, cc *grpc.ClientConn) error {
		c := bindv1.NewBindServiceClient(cc)

		_, err := c.BindProxyTarget(s.ctx, &bindv1.ProxyTargetRequest{
			Fd:     fd,
			Target: target,
		})
		return err
	})
}

func (s *Gateway) UnbindProxyTarget(gw string, fd int64, target ...string) error {
	var rqId string
	gw, rqId = s.ParseRqId(gw)
	return s.m.HostCall(s.ctx, gw, 0, s.id, "gateway", rqId, "", "", func(ctx context.Context, cc *grpc.ClientConn) error {
		c := bindv1.NewBindServiceClient(cc)

		_, err := c.UnbindProxyTarget(s.ctx, &bindv1.ProxyTargetRequest{
			Fd:     fd,
			Target: target,
		})
		return err
	})
}

type ConnInfo struct {
	LocalAddr      net.Addr
	RemoteAddr     net.Addr
	ConnectAt      time.Time
	SocketType     string
	Uid            uint32
	UName          string
	TargetType     string
	TargetId       string
	TargetCid      uint32
	TargetUid      uint32
	TargetProtocol uint32
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
	var rqId string
	var resp *connv1.ConnInfoResponse
	gw, rqId = s.ParseRqId(gw)
	err := s.m.HostCall(s.ctx, gw, 0, s.id, "gateway", rqId, "", "", func(ctx context.Context, cc *grpc.ClientConn) error {
		c := connv1.NewConnServiceClient(cc)
		var err1 error
		resp, err1 = c.Info(s.ctx, &connv1.ConnInfoRequest{
			Fd: fd,
		})
		return err1
	})

	if err != nil {
		return ConnInfo{}, err
	}

	t, _ := time.Parse("2006-01-02 15:04:05", resp.ConnectAt)
	return ConnInfo{
		LocalAddr:      Addr{net: resp.LocalNetwork, addr: resp.LocalAddr},
		RemoteAddr:     Addr{net: resp.RemoteNetwork, addr: resp.RemoteAddr},
		ConnectAt:      t,
		SocketType:     resp.SocketType,
		Uid:            resp.Uid,
		UName:          resp.Uname,
		TargetType:     resp.TargetType,
		TargetId:       resp.TargetId,
		TargetCid:      resp.TargetCid,
		TargetUid:      resp.TargetUid,
		TargetProtocol: resp.TargetProtocol,
	}, nil
}

func (s *Gateway) SendFdMessage(gw string, fd int64, act codec.Action, data codec.DataPtr) error {
	var rqId string
	gw, rqId = s.ParseRqId(gw)
	return s.m.HostCall(s.ctx, gw, 1, s.id, "gateway", rqId, "", "", func(ctx context.Context, cc *grpc.ClientConn) error {
		c := messagev1.NewMessageServiceClient(cc)

		var pbMsg []byte
		var jsonMsg []byte
		var err error
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
	})

}

func (s *Gateway) SendIdMessage(gw string, id *messagev1.SendMessageRequest_BindId, act codec.Action, data codec.DataPtr) error {
	var rqId string
	gw, rqId = s.ParseRqId(gw)
	return s.m.HostCall(s.ctx, gw, 1, s.id, "gateway", rqId, "", "", func(ctx context.Context, cc *grpc.ClientConn) error {
		c := messagev1.NewMessageServiceClient(cc)

		var pbMsg []byte
		var jsonMsg []byte
		var err error
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
	})

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
	var rqId string
	gw, rqId = s.ParseRqId(gw)
	return s.m.HostCall(s.ctx, gw, 2, s.id, "gateway", rqId, "", "", func(ctx context.Context, cc *grpc.ClientConn) error {
		c := groupv1.NewGroupServiceClient(cc)

		_, err := c.JoinGroup(s.ctx, &groupv1.JoinGroupRequest{
			Group: &groupv1.Group{Name: group},
			Member: &groupv1.Member{
				Fd: fd,
				Id: id,
			},
		})
		return err
	})

}

func (s *Gateway) LeaveGroup(gw string, group string, fd int64) error {
	var rqId string
	gw, rqId = s.ParseRqId(gw)
	return s.m.HostCall(s.ctx, gw, 2, s.id, "gateway", rqId, "", "", func(ctx context.Context, cc *grpc.ClientConn) error {
		c := groupv1.NewGroupServiceClient(cc)

		_, err := c.LeaveGroup(s.ctx, &groupv1.LeaveGroupRequest{
			Group: &groupv1.Group{Name: group},
			Fd:    fd,
		})
		return err
	})

}

func (s *Gateway) Broadcast(gw string, group string, act codec.Action, data codec.DataPtr, id string) error {
	var rqId string
	gw, rqId = s.ParseRqId(gw)
	return s.m.HostCall(s.ctx, gw, 2, s.id, "gateway", rqId, "", "", func(ctx context.Context, cc *grpc.ClientConn) error {
		c := groupv1.NewGroupServiceClient(cc)

		var pbMsg []byte
		var jsonMsg []byte
		var err error
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
	})

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
	var rqId string
	gw, rqId = s.ParseRqId(gw)
	return s.m.HostCall(s.ctx, gw, 0, s.id, "gateway", rqId, "", "", func(ctx context.Context, cc *grpc.ClientConn) error {
		c := slbv1.NewSlbServiceClient(cc)

		_, err := c.SetActionSlb(s.ctx, &slbv1.ActionSlbRequest{
			Fd:     fd,
			Action: action,
			Sbl:    slb,
		})
		return err
	})
}
