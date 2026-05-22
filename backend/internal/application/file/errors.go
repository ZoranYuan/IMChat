package file

import "errors"

var (
	ErrFileRequired       = errors.New("文件不能为空")
	ErrFileNotFound       = errors.New("文件不存在")
	ErrInvalidUpload      = errors.New("上传任务不存在")
	ErrInvalidPart        = errors.New("分片参数错误")
	ErrUploadNotCompleted = errors.New("分片未上传完成")
	ErrUploadUnauthorized = errors.New("无权操作该上传任务")
)
