package file

import (
	fileapp "IM_backend/internal/application/file"
	"IM_backend/internal/transport/http/response"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handle struct {
	app *fileapp.Application
}

func NewHandle(app *fileapp.Application) *Handle {
	return &Handle{app: app}
}

func (h *Handle) Upload(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "文件不能为空"))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "读取文件失败"))
		return
	}
	defer file.Close()

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	dto, err := h.app.Upload(c.Request.Context(), fileapp.UploadDTO{
		UploaderId:  userId,
		FileName:    fileHeader.Filename,
		ContentType: contentType,
		Size:        fileHeader.Size,
		Reader:      file,
	})
	if err != nil {
		log.Println("上传文件失败：", err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "上传文件失败"))
		return
	}

	c.JSON(http.StatusOK, response.Success(toFileRes(dto)))
}

func (h *Handle) InitMultipartUpload(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}

	var req MultipartInitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	dto, err := h.app.InitMultipartUpload(c.Request.Context(), fileapp.MultipartInitDTO{
		UploaderId:  userId,
		FileName:    req.FileName,
		ContentType: req.ContentType,
		Size:        req.Size,
		FileHash:    req.FileHash,
		ChunkSize:   req.ChunkSize,
		TotalChunks: req.TotalChunks,
	})
	if err != nil {
		log.Println("初始化分片上传失败：", err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "初始化上传失败"))
		return
	}

	c.JSON(http.StatusOK, response.Success(toMultipartInitRes(dto)))
}

func (h *Handle) UploadMultipartPart(c *gin.Context) {
	userId := c.GetString("userId")
	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
		return
	}
	uploadId := c.Param("uploadId")
	partNumber, err := strconv.Atoi(c.Param("partNumber"))
	if uploadId == "" || err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	fileHeader, err := c.FormFile("chunk")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "分片不能为空"))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "读取分片失败"))
		return
	}
	defer file.Close()

	uploadedParts, err := h.app.UploadMultipartPart(c.Request.Context(), fileapp.MultipartPartDTO{
		UploadId:   uploadId,
		UploaderId: userId,
		PartNumber: partNumber,
		ChunkHash:  c.PostForm("chunkHash"),
		Size:       fileHeader.Size,
		Reader:     file,
	})
	if err != nil {
		log.Println("上传文件分片失败：", err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "上传分片失败"))
		return
	}

	c.JSON(http.StatusOK, response.Success(MultipartPartRes{
		UploadId:      uploadId,
		PartNumber:    partNumber,
		UploadedParts: uploadedParts,
	}))
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

	dto, err := h.app.CompleteMultipartUpload(c.Request.Context(), uploadId, userId)
	if err != nil {
		if errors.Is(err, fileapp.ErrUploadNotCompleted) || errors.Is(err, fileapp.ErrInvalidUpload) {
			c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, err.Error()))
			return
		}
		log.Println("完成分片上传失败：", err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "合并文件失败"))
		return
	}

	c.JSON(http.StatusOK, response.Success(toFileRes(dto)))
}

func (h *Handle) Get(c *gin.Context) {
	fileId := c.Param("fileId")
	if fileId == "" {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	dto, err := h.app.Get(c.Request.Context(), fileId)
	if err != nil {
		if errors.Is(err, fileapp.ErrFileNotFound) {
			c.JSON(http.StatusNotFound, response.Error(http.StatusNotFound, "文件不存在"))
			return
		}
		log.Println("获取文件失败：", err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "获取文件失败"))
		return
	}

	c.JSON(http.StatusOK, response.Success(toFileRes(dto)))
}
