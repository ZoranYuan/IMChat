package file

import (
	fileapp "IM_backend/internal/application/file"
	"IM_backend/internal/transport/http/response"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handle struct {
	app *fileapp.FileApplication
}

func NewHandle(app *fileapp.FileApplication) *Handle {
	return &Handle{app: app}
}

func (h *Handle) InitDirectUpload(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}
	var req DirectUploadInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}
	dto, err := h.app.InitDirectUpload(c.Request.Context(), fileapp.DirectUploadInitDTO{
		UploaderID:  userId,
		FileName:    req.FileName,
		ContentType: req.ContentType,
		Size:        req.Size,
		FileHash:    req.FileHash,
	})
	if err != nil {
		if errors.Is(err, fileapp.ErrInvalidUpload) {
			c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, err.Error()))
			return
		}
		if errors.Is(err, fileapp.ErrFileTooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, response.Error(http.StatusRequestEntityTooLarge, err.Error()))
			return
		}
		if errors.Is(err, fileapp.ErrUploadBusy) {
			c.JSON(http.StatusConflict, response.Error(http.StatusConflict, err.Error()))
			return
		}
		log.Println("初始化直传失败：", err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "初始化上传失败"))
		return
	}
	c.JSON(http.StatusOK, response.Success(toDirectUploadInitResponse(dto)))
}

func (h *Handle) CompleteDirectUpload(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}
	uploadId := c.Param("uploadId")
	if uploadId == "" {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}
	dto, err := h.app.CompleteDirectUpload(c.Request.Context(), uploadId, userId)
	if err != nil {
		switch {
		case errors.Is(err, fileapp.ErrUploadUnauthorized):
			c.JSON(http.StatusForbidden, response.Error(http.StatusForbidden, err.Error()))
		case errors.Is(err, fileapp.ErrUploadNotCompleted):
			c.JSON(http.StatusConflict, response.Error(http.StatusConflict, err.Error()))
		case errors.Is(err, fileapp.ErrInvalidUpload):
			c.JSON(http.StatusConflict, response.Error(http.StatusConflict, err.Error()))
		case errors.Is(err, fileapp.ErrUploadBusy):
			c.JSON(http.StatusConflict, response.Error(http.StatusConflict, err.Error()))
		case errors.Is(err, fileapp.ErrFileSizeMismatch), errors.Is(err, fileapp.ErrFileHashMismatch):
			c.JSON(http.StatusUnprocessableEntity, response.Error(http.StatusUnprocessableEntity, err.Error()))
		default:
			log.Println("完成直传失败：", err)
			c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "完成上传失败"))
		}
		return
	}
	c.JSON(http.StatusOK, response.Success(toFileResponse(dto)))
}

func (h *Handle) InitMultipartUpload(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	var req MultipartInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	dto, err := h.app.InitMultipartUpload(c.Request.Context(), fileapp.MultipartInitDTO{
		UploaderID:  userId,
		FileName:    req.FileName,
		ContentType: req.ContentType,
		Size:        req.Size,
		FileHash:    req.FileHash,
		ChunkSize:   req.ChunkSize,
		TotalChunks: req.TotalChunks,
	})
	if err != nil {
		if errors.Is(err, fileapp.ErrInvalidPart) {
			c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, err.Error()))
			return
		}
		if errors.Is(err, fileapp.ErrFileTooLarge) || errors.Is(err, fileapp.ErrTooManyParts) {
			c.JSON(http.StatusRequestEntityTooLarge, response.Error(http.StatusRequestEntityTooLarge, err.Error()))
			return
		}
		if errors.Is(err, fileapp.ErrUploadBusy) {
			c.JSON(http.StatusConflict, response.Error(http.StatusConflict, err.Error()))
			return
		}
		log.Println("初始化分片上传失败：", err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "初始化上传失败"))
		return
	}

	c.JSON(http.StatusOK, response.Success(toMultipartInitResponse(dto)))
}

func (h *Handle) PresignMultipartParts(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}
	uploadId := c.Param("uploadId")
	if uploadId == "" {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}
	var req MultipartPartsPresignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "分片参数错误"))
		return
	}
	dtos, err := h.app.PresignMultipartParts(c.Request.Context(), uploadId, userId, req.PartNumbers)
	if err != nil {
		if errors.Is(err, fileapp.ErrUploadUnauthorized) {
			c.JSON(http.StatusForbidden, response.Error(http.StatusForbidden, "无权操作该上传任务"))
			return
		}
		log.Println("批量生成分片上传地址失败：", err)
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "分片上传任务无效"))
		return
	}
	res := make([]MultipartPartURLResponse, 0, len(dtos))
	for _, dto := range dtos {
		res = append(res, MultipartPartURLResponse{UploadID: dto.UploadID, PartNumber: dto.PartNumber, URL: dto.URL})
	}
	c.JSON(http.StatusOK, response.Success(res))
}

func (h *Handle) CompleteMultipartUpload(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	uploadId := c.Param("uploadId")
	if uploadId == "" {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	appDTO, err := h.app.CompleteMultipartUpload(c.Request.Context(), uploadId, userId)
	if err != nil {
		var incompleteErr *fileapp.UploadIncompleteError
		if errors.As(err, &incompleteErr) {
			c.JSON(http.StatusConflict, response.ErrorWithData(http.StatusConflict, incompleteErr.Error(), incompleteErr))
			return
		}
		if errors.Is(err, fileapp.ErrInvalidUpload) || errors.Is(err, fileapp.ErrUploadBusy) {
			c.JSON(http.StatusConflict, response.Error(http.StatusConflict, err.Error()))
			return
		}
		if errors.Is(err, fileapp.ErrUploadUnauthorized) {
			c.JSON(http.StatusForbidden, response.Error(http.StatusForbidden, "无权操作该上传任务"))
			return
		}
		log.Println("完成分片上传失败：", err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "合并文件失败"))
		return
	}

	c.JSON(http.StatusOK, response.Success(toFileResponse(appDTO)))
}

func (h *Handle) GetAttachmentAccessURL(c *gin.Context) {
	userId := c.GetString("userId")

	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	attachmentId := c.Param("attachmentId")
	if attachmentId == "" {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	dto, err := h.app.GetAttachmentAccessURL(c.Request.Context(), userId, attachmentId)
	if err != nil {
		if errors.Is(err, fileapp.ErrFileNotFound) {
			c.JSON(http.StatusNotFound, response.Error(http.StatusNotFound, "文件不存在"))
			return
		}
		if errors.Is(err, fileapp.ErrUploadUnauthorized) {
			c.JSON(http.StatusForbidden, response.Error(http.StatusForbidden, "无权访问该附件"))
			return
		}
		log.Println("获取文件失败：", err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "获取文件失败"))
		return
	}

	c.JSON(http.StatusOK, response.Success(toAttachmentAccessURLResponse(dto)))
}

func (h *Handle) GetAttachmentAccessURLs(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	var req AttachmentAccessURLsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	dtos, err := h.app.GetAttachmentAccessURLs(c.Request.Context(), userId, req.AttachmentIDs)
	if err != nil {
		if errors.Is(err, fileapp.ErrUploadUnauthorized) {
			c.JSON(http.StatusForbidden, response.Error(http.StatusForbidden, "无权访问该附件"))
			return
		}
		log.Println("批量获取附件访问地址失败：", err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "获取附件失败"))
		return
	}

	c.JSON(http.StatusOK, response.Success(toAttachmentAccessURLsResponse(dtos)))
}
