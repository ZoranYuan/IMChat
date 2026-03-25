package https_user

import (
	"IM_backend/internal/apis/response"
	application_user "IM_backend/internal/applications/user"
	user_valueobject "IM_backend/internal/domain/user/value_object"
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

// @Summary Create a mytodo
// @Description Create a mytodo by GetTodoReq (base or vip)
// @Tags todos
// @Accept json
// @Produce json
// @Param req body GetTodoReq true "create todoList request message"
// @Success 200 {string} string "ok"
// @Failure 400 {string} string "bad request"
// @Failure 500 {string} string "Internal Server Error"
// @Router /user/login [post]
func (uh *UserHandler) Login(c *gin.Context) {
	var req = UserLoginReq{}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(201, "error request"))
		return
	}

	var (
		userApp *application_user.UserAppDTO
		token   string
		err     error
	)

	switch req.LoginType {
	case int(user_valueobject.PhoneType):
		if req.Phone == "" || req.Password == "" {
			c.JSON(http.StatusBadRequest, response.Error(201, "参数错误"))
			return
		}

		userApp, token, err = uh.app.LoginWithPhone(req.Phone, req.Password)
	case int(user_valueobject.UserNameType):
	case int(user_valueobject.WxType):
		// TODO 首先判断该微信用户是否注册，如果为注册，则调用注册接口
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(201, err.Error()))
		return
	}

	var res = UserRegisterRes{
		UserId:    userApp.UserId,
		UserName:  userApp.UserName,
		NickName:  userApp.NickName,
		Phone:     userApp.Phone,
		Avatar:    userApp.Avatar,
		WxOpenID:  userApp.WxOpenID,
		WxUnionID: userApp.WxUnionID,
		Token:     token,
	}

	c.JSON(http.StatusOK, response.Success(res))
}

func (uh *UserHandler) Logout(c *gin.Context) {

}

func (uh *UserHandler) Register(c *gin.Context) {
	var req = UserRegisterReq{}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(201, "error request"))
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
			c.JSON(http.StatusBadRequest, response.Error(201, "参数错误"))
			return
		}
		userApp, token, err = uh.app.RegisterWithPhone(req.Password, req.Phone, req.ReconfirmPassword)
	case int(user_valueobject.WxType):
		// TODO 微信登录
	}

	if err != nil {
		c.JSON(201, response.Error(201, err.Error()))
		return
	}

	var res = UserRegisterRes{
		UserId:    userApp.UserId,
		UserName:  userApp.UserName,
		NickName:  userApp.NickName,
		Phone:     userApp.Phone,
		Avatar:    userApp.Avatar,
		WxOpenID:  userApp.WxOpenID,
		WxUnionID: userApp.WxUnionID,
		Token:     token,
	}

	// TODO 更新 Redis
	c.JSON(http.StatusOK, response.Success(res))
}
