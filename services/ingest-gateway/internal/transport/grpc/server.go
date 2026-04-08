package grpc

import (
	"context"
	"time"

	monitorv1 "github.com/gofxq/gaoming/api/gen/go/monitor/v1"
	"github.com/gofxq/gaoming/pkg/contracts"
	"github.com/gofxq/gaoming/services/ingest-gateway/internal/service"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	monitorv1.UnimplementedMetricsIngestServiceServer
	svc *service.Service
}

func NewServer(svc *service.Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) PushMetricBatch(_ context.Context, req *monitorv1.PushMetricBatchRequest) (*monitorv1.Ack, error) {
	resp := s.svc.PushMetricBatch(fromProtoPushMetricBatchRequest(req))
	return toProtoAck(resp), nil
}

func (s *Server) PushEventBatch(_ context.Context, req *monitorv1.PushEventBatchRequest) (*monitorv1.Ack, error) {
	resp := s.svc.PushEventBatch(fromProtoPushEventBatchRequest(req))
	return toProtoAck(resp), nil
}

func fromProtoPushMetricBatchRequest(req *monitorv1.PushMetricBatchRequest) contracts.PushMetricBatchRequest {
	points := make([]contracts.MetricPoint, 0, len(req.GetPoints()))
	for _, point := range req.GetPoints() {
		var ts time.Time
		if point.GetTs() != nil {
			ts = point.GetTs().AsTime()
		}
		points = append(points, contracts.MetricPoint{
			Name:   point.GetName(),
			Value:  point.GetValue(),
			TS:     ts,
			Labels: cloneStringMap(point.GetLabels()),
		})
	}

	collectedAt := req.GetCollectedAt()
	var collectedAtTime time.Time
	if collectedAt != nil {
		collectedAtTime = collectedAt.AsTime()
	}

	return contracts.PushMetricBatchRequest{
		HostUID:     req.GetHostUid(),
		AgentID:     req.GetAgentId(),
		BatchSeq:    req.GetBatchSeq(),
		CollectedAt: collectedAtTime,
		Points:      points,
	}
}

func fromProtoPushEventBatchRequest(req *monitorv1.PushEventBatchRequest) contracts.PushEventBatchRequest {
	events := make([]contracts.EventRecord, 0, len(req.GetEvents()))
	for _, event := range req.GetEvents() {
		var ts time.Time
		if event.GetTs() != nil {
			ts = event.GetTs().AsTime()
		}
		events = append(events, contracts.EventRecord{
			Type:     event.GetType(),
			Severity: event.GetSeverity().String(),
			Message:  event.GetMessage(),
			TS:       ts,
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

func toProtoAck(resp contracts.AckResponse) *monitorv1.Ack {
	return &monitorv1.Ack{
		RequestId:  resp.RequestID,
		Code:       int32(resp.Code),
		Message:    resp.Message,
		ServerTime: timestamppb.New(resp.ServerTime),
	}
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
