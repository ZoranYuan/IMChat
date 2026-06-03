package testdata

import (
	"IM_backend/internal/application/testdata"
	"IM_backend/internal/transport/http/response"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handle struct {
	app *testdata.BootstrapApplication
}

func NewHandle(app *testdata.BootstrapApplication) *Handle {
	return &Handle{app: app}
}

func (h *Handle) Rebuild(c *gin.Context) {
	res, err := h.app.Bootstrap(c.Request.Context())
	if err != nil {
		log.Println("failed to bootstrap testdata, ", err)
		c.JSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(res))
}
