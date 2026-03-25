package response

func Success(data interface{}) response {
	return response{
		Code:    0,
		Message: "success",
		Data:    data,
	}
}

func Error(code int, msg string) response {
	return response{
		Code:    code,
		Message: msg,
	}
}
