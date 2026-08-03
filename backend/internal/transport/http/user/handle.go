package user

import (
	userapp "IM_backend/internal/application/user"
	uservo "IM_backend/internal/domain/user/value_object"
	"IM_backend/internal/transport/http/response"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type UserHandle struct {
	app *userapp.UserApplication
}

func NewUserHandle(app *userapp.UserApplication) *UserHandle {
	return &UserHandle{
		app: app,
	}
}

func (uh *UserHandle) Login(c *gin.Context) {
	var req = UserLoginReq{}

	defer func() {
		if r := recover(); r != nil {
			log.Println("用户接口发生异常：", r)
			c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "未知错误"))
		}
	}()

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "请求参数错误"))
		return
	}

	var (
		userApp *userapp.UserAppDTO
		err     error
	)

	switch req.LoginType {
	case int(uservo.PhoneType):
		if req.Phone == "" || req.Password == "" {
			c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
			return
		}

		userApp, err = uh.app.LoginWithPhone(req.Phone, req.Password)
	case int(uservo.WxType):
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "暂不支持该登录方式"))
		return
	default:
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	if err != nil {
		switch {
		case errors.Is(err, userapp.ErrUserNotFound):
			c.JSON(http.StatusNotFound, response.Error(http.StatusNotFound, err.Error()))
		case errors.Is(err, userapp.ErrIncorrectPassword):
			c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, err.Error()))
		case errors.Is(err, userapp.ErrInvalidPhoneNumber), errors.Is(err, userapp.ErrPasswordMismatch):
			c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "登录失败"))
		}
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

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		"refresh_token",
		userApp.RefreshToken,
		7*24*3600,
		"/",
		"",
		uh.cookieSecure(c),
		true,
	)
	c.JSON(http.StatusOK, response.Success(res))
}

func (uh *UserHandle) Logout(c *gin.Context) {
	userId := c.GetString("userId")

	if userId == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "请登录之后再操作"))
		return
	}

	accessToken := c.GetHeader("Authorization")
	accessToken = strings.TrimSpace(strings.TrimPrefix(accessToken, "Bearer "))
	refreshToken, _ := c.Cookie("refresh_token")
	if err := uh.app.Logout(userId, accessToken, refreshToken); err != nil {
		log.Println("退出登录失败：", err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "退出登录失败"))
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("refresh_token", "", -1, "/", "", uh.cookieSecure(c), true)
	c.JSON(http.StatusOK, response.Success(nil))

}

func (uh *UserHandle) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "刷新令牌无效"))
		return
	}
	userApp, err := uh.app.Refresh(refreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "刷新令牌无效"))
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("refresh_token", userApp.RefreshToken, 7*24*3600, "/", "", uh.cookieSecure(c), true)
	c.JSON(http.StatusOK, response.Success(UserRegisterRes{UserId: userApp.UserId, UserName: userApp.UserName, NickName: userApp.NickName, Phone: userApp.Phone, Avatar: userApp.Avatar, Token: userApp.AccessToken}))
}

func (uh *UserHandle) Register(c *gin.Context) {
	var req = UserRegisterReq{}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "请求参数错误"))
		return
	}

	var (
		userApp *userapp.UserAppDTO
		err     error
	)
	switch req.LoginType {
	case int(uservo.PhoneType):
		// 验证密码，手机号字段
		if req.Phone == "" || req.Password == "" || req.ReconfirmPassword == "" {
			c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
			return
		}
		userApp, err = uh.app.RegisterWithPhone(req.Password, req.Phone, req.ReconfirmPassword)
	case int(uservo.WxType):
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "暂不支持该登录方式"))
		return
	default:
		c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, "参数错误"))
		return
	}

	if err != nil {
		switch {
		case errors.Is(err, userapp.ErrUserAlreadyExists):
			c.JSON(http.StatusConflict, response.Error(http.StatusConflict, err.Error()))
		case errors.Is(err, userapp.ErrInvalidPhoneNumber), errors.Is(err, userapp.ErrPasswordMismatch):
			c.JSON(http.StatusBadRequest, response.Error(http.StatusBadRequest, err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "注册失败"))
		}
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

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		"refresh_token",
		userApp.RefreshToken,
		7*24*3600,
		"/",
		"",
		uh.cookieSecure(c),
		true,
	)
	c.JSON(http.StatusOK, response.Success(res))
}

func (uh *UserHandle) cookieSecure(c *gin.Context) bool {
	return c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}

func (uh *UserHandle) GetUserByID(c *gin.Context) {
	userId := c.Param("userId")
	if userId == "" {
		c.JSON(http.StatusBadRequest, response.Error(201, "参数错误"))
		return
	}

	userApp, err := uh.app.GetUserByID(userId)

	if err != nil {
		log.Println("根据用户 ID 查询用户失败：", err)
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

func (uh *UserHandle) ResolveUser(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, response.Error(201, "参数错误"))
		return
	}

	userApp, err := uh.app.ResolveUser(keyword)
	if err != nil {
		log.Println("解析用户信息失败：", err)
		c.JSON(http.StatusNotFound, response.Error(201, "用户不存在"))
		return
	}

	c.JSON(http.StatusOK, response.Success(UserInfoRes{
		UserId:   userApp.UserId,
		UserName: userApp.UserName,
		NickName: userApp.NickName,
		Phone:    userApp.Phone,
		Avatar:   userApp.Avatar,
	}))
}
