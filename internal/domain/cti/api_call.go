package cti

import (
	"context"
	"errors"
	"log/slog"

	"yunshu/internal/contracts"
	"yunshu/internal/infra/events"
)

var ErrInvalidApiCall = errors.New("invalid api call request")

// ESLClient 是 CTI 调用 ESL 起呼能力的端口。
// 部署在同一 cc-call 进程时可以是内存调用，拆分部署时可以替换为 HTTP/gRPC client。
type ESLClient interface {
	StartAPIOutbound(ctx context.Context, version, callID string, req contracts.ApiCallReq) error
	StartBatchOutbound(ctx context.Context, version, callID string, req contracts.BatchCallReq) error
}

// APICallService 处理  `/cti/callTask/call` 对应的 API 外呼业务入口。
type APICallService struct {
	ESL    ESLClient
	Events events.Bus
	Logger *slog.Logger
}

// Run 校验 API 外呼请求并提交 ESL 起呼。
func (s *APICallService) Run(ctx context.Context, version, callID string, req contracts.ApiCallReq) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("开始处理 CTI API 外呼请求", "callId", callID, "userId", req.UserID)
	if callID == "" || req.Callee == "" || req.UserID == 0 {
		logger.Warn("CTI API 外呼请求参数不完整", "callId", callID, "userId", req.UserID)
		return ErrInvalidApiCall
	}
	if s.Events != nil {
		if err := s.Events.Publish(ctx, contracts.NewEventEnvelope(
			"api-call-requested:"+callID,
			contracts.EventAPICallRequested,
			"api-call:"+callID,
			"call",
			callID,
			contracts.ServiceCall,
			map[string]any{"callId": callID, "userId": req.UserID, "callee": req.Callee},
		)); err != nil {
			logger.Error("CTI API 外呼请求事件发布失败", "callId", callID, "userId", req.UserID, "error", err.Error())
			return err
		}
	}
	if err := s.ESL.StartAPIOutbound(ctx, version, callID, req); err != nil {
		logger.Error("CTI 调用 ESL API 外呼失败", "callId", callID, "userId", req.UserID, "error", err.Error())
		return err
	}
	logger.Info("CTI API 外呼请求处理完成", "callId", callID, "userId", req.UserID)
	return nil
}
