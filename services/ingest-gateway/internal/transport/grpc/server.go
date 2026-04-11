package grpc

import (
	"context"
	"errors"
	"io"
	"time"

	monitorv1 "github.com/gofxq/gaoming/api/gen/go/monitor/v1"
	"github.com/gofxq/gaoming/pkg/contracts"
	hostruntime "github.com/gofxq/gaoming/pkg/hostruntime/repository"
	"github.com/gofxq/gaoming/services/ingest-gateway/internal/service"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	monitorv1.UnimplementedAgentControlServiceServer
	monitorv1.UnimplementedMetricsIngestServiceServer
	svc *service.Service
}

func NewServer(svc *service.Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) RegisterAgent(ctx context.Context, req *monitorv1.RegisterAgentRequest) (*monitorv1.RegisterAgentResponse, error) {
	resp, err := s.svc.RegisterAgent(ctx, fromProtoRegisterAgentRequest(req))
	if err != nil {
		return nil, grpcstatus.Error(codes.Internal, err.Error())
	}
	return toProtoRegisterAgentResponse(resp), nil
}

func (s *Server) PushMetricBatch(ctx context.Context, req *monitorv1.PushMetricBatchRequest) (*monitorv1.Ack, error) {
	resp, err := s.svc.PushMetricBatch(ctx, fromProtoPushMetricBatchRequest(req))
	if err != nil {
		if err == hostruntime.ErrHostNotFound {
			return nil, grpcstatus.Error(codes.NotFound, err.Error())
		}
		return nil, grpcstatus.Error(codes.Internal, err.Error())
	}
	return toProtoAck(resp), nil
}

func (s *Server) StreamMetricBatches(stream monitorv1.MetricsIngestService_StreamMetricBatchesServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) || grpcstatus.Code(err) == codes.Canceled {
				return nil
			}
			return err
		}

		resp, err := s.svc.PushMetricBatch(stream.Context(), fromProtoPushMetricBatchRequest(req))
		if err != nil {
			if err == hostruntime.ErrHostNotFound {
				return grpcstatus.Error(codes.NotFound, err.Error())
			}
			return grpcstatus.Error(codes.Internal, err.Error())
		}

		if err := stream.Send(&monitorv1.MetricBatchAck{
			Ack:      toProtoAck(resp),
			BatchSeq: req.GetBatchSeq(),
		}); err != nil {
			return err
		}
	}
}

func (s *Server) PushEventBatch(_ context.Context, req *monitorv1.PushEventBatchRequest) (*monitorv1.Ack, error) {
	resp := s.svc.PushEventBatch(fromProtoPushEventBatchRequest(req))
	return toProtoAck(resp), nil
}

func fromProtoRegisterAgentRequest(req *monitorv1.RegisterAgentRequest) contracts.RegisterAgentRequest {
	return contracts.RegisterAgentRequest{
		Host: contracts.HostIdentity{
			HostUID:    req.GetHost().GetHostUid(),
			TenantCode: req.GetHost().GetTenantCode(),
			Hostname:   req.GetHost().GetHostname(),
			PrimaryIP:  req.GetHost().GetPrimaryIp(),
			IPs:        append([]string(nil), req.GetHost().GetIps()...),
			OSType:     req.GetHost().GetOsType(),
			Arch:       req.GetHost().GetArch(),
			Region:     req.GetHost().GetRegion(),
			AZ:         req.GetHost().GetAz(),
			Env:        req.GetHost().GetEnv(),
			Role:       req.GetHost().GetRole(),
			Labels:     cloneStringMap(req.GetHost().GetLabels()),
		},
		Agent: contracts.AgentMetadata{
			AgentID:      req.GetAgent().GetAgentId(),
			Version:      req.GetAgent().GetVersion(),
			Capabilities: append([]string(nil), req.GetAgent().GetCapabilities()...),
			BootTime:     asTime(req.GetAgent().GetBootTime()),
		},
	}
}

func fromProtoPushMetricBatchRequest(req *monitorv1.PushMetricBatchRequest) contracts.PushMetricBatchRequest {
	points := make([]contracts.MetricPoint, 0, len(req.GetPoints()))
	for _, point := range req.GetPoints() {
		points = append(points, contracts.MetricPoint{
			Name:   point.GetName(),
			Value:  point.GetValue(),
			TS:     asTime(point.GetTs()),
			Labels: cloneStringMap(point.GetLabels()),
		})
	}

	return contracts.PushMetricBatchRequest{
		HostUID:     req.GetHostUid(),
		AgentID:     req.GetAgentId(),
		BatchSeq:    req.GetBatchSeq(),
		CollectedAt: asTime(req.GetCollectedAt()),
		Points:      points,
	}
}

func fromProtoPushEventBatchRequest(req *monitorv1.PushEventBatchRequest) contracts.PushEventBatchRequest {
	events := make([]contracts.EventRecord, 0, len(req.GetEvents()))
	for _, event := range req.GetEvents() {
		events = append(events, contracts.EventRecord{
			Type:     event.GetType(),
			Severity: event.GetSeverity().String(),
			Message:  event.GetMessage(),
			TS:       asTime(event.GetTs()),
			Attrs:    cloneStringMap(event.GetAttrs()),
		})
	}

	return contracts.PushEventBatchRequest{
		HostUID:  req.GetHostUid(),
		AgentID:  req.GetAgentId(),
		BatchSeq: req.GetBatchSeq(),
		Events:   events,
	}
}

func toProtoRegisterAgentResponse(resp contracts.RegisterAgentResponse) *monitorv1.RegisterAgentResponse {
	return &monitorv1.RegisterAgentResponse{
		Ack:        toProtoAck(contracts.AckResponse{RequestID: resp.RequestID, Code: 0, Message: resp.Message, ServerTime: time.Now().UTC()}),
		HostUid:    resp.HostUID,
		Config:     toProtoAgentConfig(resp.Config),
		TenantCode: resp.TenantCode,
	}
}

func toProtoAck(resp contracts.AckResponse) *monitorv1.Ack {
	return &monitorv1.Ack{
		RequestId:  resp.RequestID,
		Code:       int32(resp.Code),
		Message:    resp.Message,
		ServerTime: timestamppb.New(resp.ServerTime),
	}
}

func toProtoAgentConfig(cfg contracts.AgentConfig) *monitorv1.AgentConfig {
	if cfg.ConfigVersion == 0 && cfg.HeartbeatIntervalSec == 0 && cfg.MetricIntervalSec == 0 && len(cfg.StaticLabels) == 0 {
		return nil
	}
	return &monitorv1.AgentConfig{
		ConfigVersion:        cfg.ConfigVersion,
		HeartbeatIntervalSec: int32(cfg.HeartbeatIntervalSec),
		MetricIntervalSec:    int32(cfg.MetricIntervalSec),
		StaticLabels:         cloneStringMap(cfg.StaticLabels),
	}
}

func asTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}
