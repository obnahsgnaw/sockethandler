package impl

import (
	"context"
	"errors"
	"github.com/obnahsgnaw/sockethandler/service/action"
	"github.com/obnahsgnaw/sockethandler/service/codec"
	handlerv1 "github.com/obnahsgnaw/sockethandler/service/proto/gen/handler/v1"
	"go.uber.org/zap"
	"strconv"
)

type HandlerService struct {
	manager *action.Manager
	logger  *zap.Logger
	handlerv1.UnimplementedHandlerServiceServer
}

func NewHandlerService(manager *action.Manager, logger *zap.Logger) *HandlerService {
	return &HandlerService{manager: manager, logger: logger}
}

func (s *HandlerService) Handle(ctx context.Context, q *handlerv1.HandleRequest) (*handlerv1.HandleResponse, error) {
	if s.logger != nil {
		s.logger.Debug("handle request", zap.Uint32("action_id", q.ActionId), zap.String("gateway", q.Gateway), zap.Int64("fd", q.Fd), zap.String("bind_id", q.Id), zap.String("bind_type", q.Type), zap.ByteString("data", q.Package), zap.String("format", q.Format.String()))
	}
	if _, ok := s.manager.GetAction(codec.ActionId(q.ActionId)); !ok {
		s.logger.Error("not found action:" + strconv.Itoa(int(q.ActionId)))
		return nil, errors.New("NotFound")
	}
	respAction, respData, err := s.manager.Dispatch(ctx, &action.HandlerReq{
		ActionId: q.ActionId,
		Gateway:  q.Gateway,
		Fd:       q.Fd,
		Id:       q.Id,
		Type:     q.Type,
		Format:   q.Format.String(),
		Package:  q.Package,
	})
	if err != nil {
		s.logger.Error("handle failed:" + strconv.Itoa(int(q.ActionId)) + ",err=" + err.Error())
		return nil, err
	}
	s.logger.Debug("handle response", zap.String("action_id", respAction.Id.String()), zap.ByteString("data", respData))
	return &handlerv1.HandleResponse{
		ActionId:   uint32(respAction.Id),
		ActionName: respAction.Name,
		Package:    respData,
	}, nil
}
