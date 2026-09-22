package grpcagent

import (
	agentport "IM_backend/internal/application/ports/agent"
	agentpb "IM_backend/room_agent/v1"
	"context"
	"fmt"
	"time"

	oldtimestamp "github.com/golang/protobuf/ptypes/timestamp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn   *grpc.ClientConn
	client agentpb.RoomSummaryAgentClient
}

func NewClient(endpoint string) (*Client, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("摘要 Agent 地址不能为空")
	}
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, client: agentpb.NewRoomSummaryAgentClient(conn)}, nil
}

func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) RunSummary(ctx context.Context, req agentport.SummaryRequest) (*agentport.SummaryResponse, error) {
	response, err := c.client.RunSummary(ctx, toProtoRequest(req))
	if err != nil {
		return nil, err
	}
	return fromProtoResponse(response), nil
}

func (c *Client) ResumeSummaryState(ctx context.Context, req agentport.ResumeSummaryRequest) (*agentport.SummaryResponse, error) {
	response, err := c.client.ResumeSummaryState(ctx, &agentpb.ResumeSummaryStateRequest{
		RequestId:    req.RequestID,
		SummaryRunId: req.SummaryRunID,
		Scope:        &agentpb.Scope{RoomId: req.Scope.RoomID, UserId: req.Scope.UserID},
	})
	if err != nil {
		return nil, err
	}
	return fromProtoResponse(response), nil
}

func toProtoRequest(req agentport.SummaryRequest) *agentpb.SummaryRequest {
	messages := make([]*agentpb.RoomMessage, 0, len(req.RoomMessages))
	for _, item := range req.RoomMessages {
		messages = append(messages, &agentpb.RoomMessage{
			RoomId:    item.RoomID,
			MessageId: item.MessageID,
			SenderId:  item.SenderID,
			Seq:       item.Seq,
			Type:      item.Type,
			Content:   item.Content,
			Status:    item.Status,
			SendTime:  toTimestamp(item.SendTime),
		})
	}
	return &agentpb.SummaryRequest{
		RequestId:    req.RequestID,
		SummaryRunId: req.SummaryRunID,
		Scope:        &agentpb.Scope{RoomId: req.Scope.RoomID, UserId: req.Scope.UserID},
		Action:       toProtoAction(req.Action),
		RoomMessages: messages,
	}
}

func toTimestamp(value time.Time) *oldtimestamp.Timestamp {
	if value.IsZero() {
		return nil
	}
	return &oldtimestamp.Timestamp{Seconds: value.Unix(), Nanos: int32(value.Nanosecond())}
}

func toProtoAction(action agentport.SummaryAction) agentpb.SummaryAction {
	switch action {
	case agentport.SummaryActionStart:
		return agentpb.SummaryAction_SUMMARY_ACTION_START
	case agentport.SummaryActionRetry:
		return agentpb.SummaryAction_SUMMARY_ACTION_RETRY
	case agentport.SummaryActionCancel:
		return agentpb.SummaryAction_SUMMARY_ACTION_CANCEL
	default:
		return agentpb.SummaryAction_SUMMARY_ACTION_UNSPECIFIED
	}
}

func fromProtoResponse(value *agentpb.SummaryResponse) *agentport.SummaryResponse {
	response := &agentport.SummaryResponse{
		RequestID:    value.GetRequestId(),
		SummaryRunID: value.GetSummaryRunId(),
		Status:       fromProtoStatus(value.GetStatus()),
	}
	if value.GetSummary() != nil {
		response.Summary = &agentport.SummaryOutput{}
		for _, segment := range value.GetSummary().GetSummaryContent() {
			response.Summary.SummaryContent = append(response.Summary.SummaryContent, fromProtoSegment(segment))
		}
	}
	if value.GetError() != nil {
		response.Error = &agentport.SummaryError{Code: value.GetError().GetCode(), Message: value.GetError().GetMessage()}
	}
	if value.GetRetryRequired() != nil {
		retry := value.GetRetryRequired()
		response.RetryRequired = &agentport.RetryRequired{
			ReasonCode:       retry.GetReasonCode(),
			ValidationIssues: retry.GetValidationIssues(),
			ReflectionIssues: retry.GetReflectionIssues(),
			RepairCount:      retry.GetRepairCount(),
			MaxRepairCount:   retry.GetMaxRepairCount(),
		}
		for _, action := range retry.GetAllowedActions() {
			response.RetryRequired.AllowedActions = append(response.RetryRequired.AllowedActions, action.String())
		}
	}
	return response
}

func fromProtoSegment(value *agentpb.SummarySegment) agentport.SummarySegment {
	segment := agentport.SummarySegment{SummaryContentSegment: value.GetSummaryContentSegment()}
	for _, citation := range value.GetCitations() {
		segment.Citations = append(segment.Citations, agentport.Citation{MessageID: citation.GetMessageId(), Seq: citation.GetSeq()})
	}
	return segment
}

func fromProtoStatus(value agentpb.SummaryStatus) agentport.SummaryStatus {
	switch value {
	case agentpb.SummaryStatus_SUMMARY_STATUS_SUCCEEDED:
		return agentport.SummaryStatusSucceeded
	case agentpb.SummaryStatus_SUMMARY_STATUS_FAILED:
		return agentport.SummaryStatusFailed
	case agentpb.SummaryStatus_SUMMARY_STATUS_WAITING_USER_DECISION:
		return agentport.SummaryStatusWaitingUserDecision
	default:
		return "UNSPECIFIED"
	}
}
