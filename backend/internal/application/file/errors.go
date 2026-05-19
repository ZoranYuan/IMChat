package file

import "errors"

var (
	ErrFileRequired = errors.New("文件不能为空")
	ErrFileNotFound = errors.New("文件不存在")
)
