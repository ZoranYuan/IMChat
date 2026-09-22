package agent

import (
	summaryapp "IM_backend/internal/application/agent"
	agentport "IM_backend/internal/application/ports/agent"
	"IM_backend/internal/transport/http/response"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Handle struct{ app summaryapp.AgentApplication }

func NewHandle(app summaryapp.AgentApplication) *Handle { return &Handle{app: app} }

func writeSSE(writer interface{ Write([]byte) (int, error) }, eventName, eventID string, data any) error {
	if eventID != "" {
		if _, err := fmt.Fprintf(writer, "id: %s\n", eventID); err != nil {
			return err
		}
	}
	if eventName != "" {
		if _, err := fmt.Fprintf(writer, "event: %s\n", eventName); err != nil {
			return err
		}
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "data: %s\n\n", payload)
	return err
}

func (h *Handle) InitSummaryRun(c *gin.Context) {
	roomID, userID := strings.TrimSpace(c.Param("roomId")), strings.TrimSpace(c.GetString("userId"))
	if roomID == "" || userID == "" {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}
	run, err := h.app.InitSummaryRun(c.Request.Context(), roomID, userID)
	if err != nil {
		h.writeApplicationError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, response.Success(run))
}

func (h *Handle) AgentStream(c *gin.Context) {
	roomID, summaryRunID, userID := strings.TrimSpace(c.Param("roomId")), strings.TrimSpace(c.Param("summaryRunId")), strings.TrimSpace(c.GetString("userId"))
	if roomID == "" || summaryRunID == "" || userID == "" {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}
	stream, err := h.app.OpenSummarySSE(c.Request.Context(), roomID, userID, summaryRunID)
	if err != nil {
		h.writeApplicationError(c, err)
		return
	}
	defer stream.Close()
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "当前响应不支持 SSE"))
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	if err := writeSSE(c.Writer, "ready", "", map[string]any{"summaryRunId": summaryRunID}); err != nil {
		return
	}
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(c.Writer, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event, ok := <-stream.Stream:
			if !ok {
				return
			}
			if err := writeSSE(c.Writer, event.Name, "", event.Data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (h *Handle) RetrySummary(c *gin.Context)  { h.action(c, agentport.SummaryActionRetry) }
func (h *Handle) CancelSummary(c *gin.Context) { h.action(c, agentport.SummaryActionCancel) }

func (h *Handle) action(c *gin.Context, action agentport.SummaryAction) {
	roomID, summaryRunID, userID := strings.TrimSpace(c.Param("roomId")), strings.TrimSpace(c.Param("summaryRunId")), strings.TrimSpace(c.GetString("userId"))
	var err error
	if action == agentport.SummaryActionRetry {
		err = h.app.RetrySummary(c.Request.Context(), roomID, userID, summaryRunID)
	} else {
		err = h.app.CancelSummary(c.Request.Context(), roomID, userID, summaryRunID)
	}
	if err != nil {
		h.writeApplicationError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, response.Success(map[string]any{"summaryRunId": summaryRunID}))
}

func (h *Handle) writeApplicationError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, summaryapp.ErrSummaryRunNotFound):
		status = http.StatusNotFound
	case errors.Is(err, summaryapp.ErrSummaryRunInitializing):
		status = http.StatusConflict
	case errors.Is(err, summaryapp.ErrSummaryForbidden):
		status = http.StatusForbidden
	case errors.Is(err, summaryapp.ErrSummaryInvalidState):
		status = http.StatusConflict
	}
	c.JSON(status, response.Error(status, err.Error()))
}
