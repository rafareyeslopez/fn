package server

import (
	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (s *Server) handleFnPut(c *gin.Context) {
	ctx := c.Request.Context()
	fn := &models.Fn{}
	err := c.BindJSON(&fn)
	if err != nil {
		if !models.IsAPIError(err) {
			// TODO this error message sucks
			err = models.ErrInvalidJSON
		}
		handleErrorResponse(c, err)
		return
	}

	fnId := c.Param(api.FnID)

	if fnId != fn.ID {
		handleErrorResponse(c, models.ErrIDMismatch)
	}

	fn, err = s.datastore.UpdateFn(ctx, fn)

	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, fn)
}