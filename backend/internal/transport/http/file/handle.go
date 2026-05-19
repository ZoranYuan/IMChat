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
		log.Println("failed to upload file:", err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "上传文件失败"))
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
		log.Println("failed to get file:", err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "获取文件失败"))
		return
	}

	c.JSON(http.StatusOK, response.Success(toFileRes(dto)))
}
