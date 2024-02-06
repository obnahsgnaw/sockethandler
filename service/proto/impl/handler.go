package impl

import (
	"context"
	handlerv1 "github.com/obnahsgnaw/socketapi/gen/handler/v1"
	"github.com/obnahsgnaw/sockethandler/service/action"
	"github.com/obnahsgnaw/socketutil/codec"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HandlerService struct {
	manager             *ManagerProvider
	dateBuilderProvider codec.DataBuilderProvider
	handlerv1.UnimplementedHandlerServiceServer
}

func NewHandlerService(manager *ManagerProvider) *HandlerService {
	return &HandlerService{manager: manager, dateBuilderProvider: codec.NewDbp()}
}

func toCodecName(format string) codec.Name {
	if format == "json" {
		return codec.Json
	}
	if format == "proto" {
		return codec.Proto
	}

	return codec.Proto
}

func (s *HandlerService) Handle(ctx context.Context, q *handlerv1.HandleRequest) (*handlerv1.HandleResponse, error) {
	// fetch action handler
	act, structure, handler, ok := s.manager.GetManager(q.Typ).GetHandler(codec.ActionId(q.ActionId))
	if !ok {
		return nil, status.Error(codes.NotFound, "not found")
	}
	// unpack data
	data := structure()
	if data != nil {
		if err := s.dateBuilderProvider.Provider(toCodecName(q.Format)).Unpack(q.Package, data); err != nil {
			return nil, status.Error(codes.InvalidArgument, "data unpack failed, err="+err.Error())
		}
	}

	// handle
	var u *action.User
	if q.User != nil {
		u = &action.User{
			Id:   uint32(int(q.User.Id)),
			Cid:  uint32(int(q.User.Cid)),
			Oid:  uint32(int(q.User.Oid)),
			Name: q.User.Name,
		}
	}
	req := action.NewHandlerReq(q.Gateway, act, q.Fd, u, data, q.BindIds)

	respAction, respData, err := handler(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	// response data pack
	resp, err := s.dateBuilderProvider.Provider(toCodecName(q.Format)).Pack(respData)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	// response
	return &handlerv1.HandleResponse{
		ActionId:   uint32(respAction.Id),
		ActionName: respAction.Name,
		Package:    resp,
	}, nil
}
