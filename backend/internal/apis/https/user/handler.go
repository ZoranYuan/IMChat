package https_user

import (
	"IM_backend/internal/apis/response"
	application_user "IM_backend/internal/applications/user"
	user_valueobject "IM_backend/internal/domain/user/value_object"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	app *application_user.UserApplication
}

func NewUserHandler(app *application_user.UserApplication) *UserHandler {
	return &UserHandler{
		app: app,
	}
}

func (uh *UserHandler) Login(c *gin.Context) {
	var req = UserLoginReq{}

	defer func() {
		if r := recover(); r != nil {
			log.Println("panic, ", r)
		}

		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "未知错误"))
	}()

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "error request"))
		return
	}

	var (
		userApp *application_user.UserAppDTO
		err     error
	)

	switch req.LoginType {
	case int(user_valueobject.PhoneType):
		if req.Phone == "" || req.Password == "" {
			c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
			return
		}

		userApp, err = uh.app.LoginWithPhone(req.Phone, req.Password)
	case int(user_valueobject.UserNameType):
	case int(user_valueobject.WxType):
		// TODO 首先判断该微信用户是否注册，如果为注册，则调用注册接口
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, err.Error()))
		return
	}

	var res = UserRegisterRes{
		UserId:   userApp.UserId,
		UserName: userApp.UserName,
		NickName: userApp.NickName,
		Phone:    userApp.Phone,
		Avatar:   userApp.Avatar,
		Token:    userApp.AccessToken,
	}

	c.JSON(http.StatusOK, response.Success(res))

	c.SetCookie(
		"refresh_token",
		userApp.RefreshToken,
		7*24*3600,
		"/",
		"",
		false,
		true,
	)
}

func (uh *UserHandler) Logout(c *gin.Context) {
	userId := c.GetString("userId")

	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "请登录之后再操作"))
		return
	}

	if err := uh.app.Logout(userId); err != nil {
		log.Println("failed to logout, ", err)
		return
	}

	c.JSON(http.StatusOK, response.Success(nil))

}

func (uh *UserHandler) Register(c *gin.Context) {
	var req = UserRegisterReq{}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "error request"))
		return
	}

	var (
		userApp *application_user.UserAppDTO
		token   string
		err     error
	)
	switch req.LoginType {
	case int(user_valueobject.PhoneType):
		// 验证密码，手机号字段
		if req.Phone == "" || req.Password == "" || req.ReconfirmPassword == "" {
			c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
			return
		}
		userApp, err = uh.app.RegisterWithPhone(req.Password, req.Phone, req.ReconfirmPassword)
	case int(user_valueobject.WxType):
		// TODO 微信登录
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(201, err.Error()))
		return
	}

	var res = UserRegisterRes{
		UserId:   userApp.UserId,
		UserName: userApp.UserName,
		NickName: userApp.NickName,
		Phone:    userApp.Phone,
		Avatar:   userApp.Avatar,
		Token:    token,
	}

	// TODO 更新 Redis
	c.JSON(http.StatusOK, response.Success(res))

	c.SetCookie(
		"refresh_token",
		userApp.RefreshToken,
		7*24*3600,
		"/",
		"",
		false,
		true,
	)
}

func (uh *UserHandler) GetUserByUserId(c *gin.Context) {
	userId := c.Param("userId")
	if userId == "" {
		c.JSON(http.StatusBadRequest, response.Error(201, "参数错误"))
		return
	}

	userApp, err := uh.app.GetUserByUserId(userId)

	if err != nil {
		log.Println("failed to get user by userId, ", err)
		c.JSON(http.StatusInternalServerError, response.Error(201, "获取失败"))
		return
	}

	var userRes = UserInfoRes{
		UserId:   userApp.UserId,
		UserName: userApp.UserName,
		NickName: userApp.NickName,
		Phone:    userApp.Phone,
		Avatar:   userApp.Avatar,
	}

	c.JSON(http.StatusOK, response.Success(userRes))
}
