package https_user

import (
	"IM_backend/internal/applications"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	app *applications.UserApplication
}

func NewUserHandler(app *applications.UserApplication) *UserHandler {
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
func (uh *UserHandler) Login(c *gin.Context) {}
