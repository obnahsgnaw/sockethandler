package impl

import (
	"context"
	"github.com/obnahsgnaw/application/pkg/utils"
	handlerv1 "github.com/obnahsgnaw/socketapi/gen/handler/v1"
	"github.com/obnahsgnaw/sockethandler/service/action"
	"github.com/obnahsgnaw/socketutil/codec"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strconv"
)

type HandlerService struct {
	manager             *action.Manager
	logger              *zap.Logger
	dateBuilderProvider codec.DataBuilderProvider
	handlerv1.UnimplementedHandlerServiceServer
}

func NewHandlerService(manager *action.Manager, logger *zap.Logger) *HandlerService {
	return &HandlerService{manager: manager, logger: logger, dateBuilderProvider: codec.NewDbp()}
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
	if s.logger != nil {
		s.logger.Debug("handle request", zap.Uint32("action_id", q.ActionId), zap.String("gateway", q.Gateway), zap.Int64("fd", q.Fd), zap.String("bind_id", utils.ToJson(q.BindIds)), zap.ByteString("data", q.Package), zap.String("format", q.Format))
	}
	// fetch action handler
	act, structure, handler, ok := s.manager.GetHandler(codec.ActionId(q.ActionId))
	if !ok {
		s.logger.Error("no handler for action:" + strconv.Itoa(int(q.ActionId)))
		return nil, status.Error(codes.NotFound, "not found")
	}
	s.logger.Info("handle action:" + act.String())
	// unpack data
	data := structure()
	if data != nil {
		if err := s.dateBuilderProvider.Provider(toCodecName(q.Format)).Unpack(q.Package, data); err != nil {
			s.logger.Error("unpack data failed, err=" + err.Error())
			return nil, status.Error(codes.InvalidArgument, "data unpack failed, err="+err.Error())
		}
	}

	// handle
	var u *action.User
	if q.User != nil {
		u = &action.User{
			Id:   uint32(int(q.User.Id)),
			Name: q.User.Name,
		}
	}
	req := action.NewHandlerReq(q.Gateway, act, q.Fd, u, data, q.BindIds)

	respAction, respData, err := handler(ctx, req)
	if err != nil {
		s.logger.Error("handle failed, err=" + err.Error())
		return nil, status.Error(codes.Internal, err.Error())
	}
	// response data pack
	resp, err := s.dateBuilderProvider.Provider(toCodecName(q.Format)).Pack(respData)
	if err != nil {
		s.logger.Error("handle response data pack failed, err=" + err.Error())
		return nil, status.Error(codes.Internal, err.Error())
	}
	// response
	s.logger.Info("handle complete", zap.String("action", respAction.Id.String()), zap.ByteString("data", resp))
	return &handlerv1.HandleResponse{
		ActionId:   uint32(respAction.Id),
		ActionName: respAction.Name,
		Package:    resp,
	}, nil
}
