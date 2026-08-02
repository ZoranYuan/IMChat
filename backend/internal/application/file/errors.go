package file

import (
	"errors"
	"fmt"
)

type UploadIncompleteError struct {
	MissingParts []int `json:"missingParts"`
	InvalidParts []int `json:"invalidParts"`
}

func (e *UploadIncompleteError) Error() string {
	return fmt.Sprintf("分片未上传完成，缺失 %d 个，异常 %d 个", len(e.MissingParts), len(e.InvalidParts))
}

func (e *UploadIncompleteError) Unwrap() error {
	return ErrUploadNotCompleted
}

var (
	ErrFileRequired       = errors.New("文件不能为空")
	ErrFileNotFound       = errors.New("文件不存在")
	ErrFileSizeMismatch   = errors.New("文件大小校验失败")
	ErrFileHashMismatch   = errors.New("文件哈希校验失败")
	ErrDuplicateUpload    = errors.New("重复上传文件")
	ErrInvalidUpload      = errors.New("上传任务不存在")
	ErrInvalidPart        = errors.New("分片参数错误")
	ErrUploadNotCompleted = errors.New("分片未上传完成")
	ErrUploadUnauthorized = errors.New("无权操作该上传任务")
	ErrUploadBusy         = errors.New("文件正在上传")
	ErrMultipartLockLost  = errors.New("分片合并锁已失效")
)
