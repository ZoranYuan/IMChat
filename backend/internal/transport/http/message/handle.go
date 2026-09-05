package message

import (
	messageapp "IM_backend/internal/application/message"
	"IM_backend/internal/transport/http/response"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type MessageHandle struct {
	app *messageapp.MessageApplication
}

func NewMessageHandle(app *messageapp.MessageApplication) *MessageHandle {
	return &MessageHandle{
		app: app,
	}
}

func (mh *MessageHandle) GetHistoryMessages(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}
	var req MessageHistoryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}
	messagesApp, nextCursor, hasMore, err := mh.app.GetHistoryMessages(c.Request.Context(), req.ConversationID, userId, req.Limit, req.Cursor)
	if err != nil {
		switch {
		case errors.Is(err, messageapp.ErrConversationNotFound):
			c.JSON(http.StatusNotFound, response.Error(http.StatusNotFound, "会话不存在"))
		case errors.Is(err, messageapp.ErrForbidden):
			c.JSON(http.StatusForbidden, response.Error(http.StatusForbidden, "无权查看该会话"))
		default:
			c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "获取消息失败，请稍后再试"))
		}
		return
	}
	c.JSON(http.StatusOK, response.Success(toHistoryMessageResponse(messagesApp, nextCursor, hasMore)))
}

func (mh *MessageHandle) SyncMessages(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	var req MessageSyncRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	if req.ConversationID == "" {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	messagesApp, err := mh.app.SyncMessages(
		c.Request.Context(),
		req.ConversationID,
		userId,
		req.AfterSeq,
	)
	if err != nil {
		switch {
		case errors.Is(err, messageapp.ErrConversationNotFound):
			c.JSON(http.StatusNotFound, response.Error(http.StatusNotFound, "会话不存在"))
		case errors.Is(err, messageapp.ErrForbidden):
			c.JSON(http.StatusForbidden, response.Error(http.StatusForbidden, "无权查看该会话"))
		default:
			c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "同步消息失败，请稍后再试"))
		}
		return
	}

	c.JSON(http.StatusOK, response.Success(toSyncMessageResponse(messagesApp)))
}

func (mh *MessageHandle) GetMessagesBySeqs(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	var req MessageSeqsRequest
	if err := c.ShouldBindQuery(&req); err != nil || req.ConversationID == "" {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	seqs, err := parseMessageSeqs(req.Seqs)
	if err != nil || len(seqs) == 0 || len(seqs) > 500 {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	messagesApp, err := mh.app.GetMessagesBySeqs(c.Request.Context(), req.ConversationID, userId, seqs)
	if err != nil {
		switch {
		case errors.Is(err, messageapp.ErrConversationNotFound):
			c.JSON(http.StatusNotFound, response.Error(http.StatusNotFound, "会话不存在"))
		case errors.Is(err, messageapp.ErrForbidden):
			c.JSON(http.StatusForbidden, response.Error(http.StatusForbidden, "无权查看该会话"))
		default:
			c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "查询消息失败，请稍后再试"))
		}
		return
	}

	c.JSON(http.StatusOK, response.Success(toMessageSeqsResponse(messagesApp)))
}

func parseMessageSeqs(raw string) ([]int64, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	seqs := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{}, len(parts))
	for _, part := range parts {
		seq, err := strconv.ParseInt(part, 10, 64)
		if err != nil || seq <= 0 {
			return nil, err
		}
		if _, ok := seen[seq]; ok {
			continue
		}
		seen[seq] = struct{}{}
		seqs = append(seqs, seq)
	}
	return seqs, nil
}

func (mh *MessageHandle) GetVideoDanmaku(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	var req DanmakuRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	items, err := mh.app.GetVideoDanmaku(
		c.Request.Context(),
		req.RoomID,
		userId,
		req.VideoID,
		req.StartTime,
		req.EndTime,
		req.Limit,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "获取弹幕失败，请稍后再试"))
		return
	}

	c.JSON(http.StatusOK, response.Success(toDanmakuResponse(items)))
}

func (mh *MessageHandle) GetRoomVideoHistory(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	var req RoomVideoHistoryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	items, err := mh.app.GetRoomVideoHistory(c.Request.Context(), req.RoomID, userId, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "获取历史视频失败，请稍后再试"))
		return
	}

	c.JSON(http.StatusOK, response.Success(toRoomVideoHistoryResponse(items)))
}
