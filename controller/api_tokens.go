// controller/api_tokens.go
package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

type createTokenReq struct {
	Name      string     `json:"name"`
	Scope     string     `json:"scope"`
	ExpiresAt *time.Time `json:"expires_at"`
}
type createTokenResp struct {
	ID     uint   `json:"id"`
	Prefix string `json:"prefix"`
	Token  string `json:"token"` // nur einmalig!
}

func (ctrl *controller) apiCreateToken(c echo.Context) error {
	var req createTokenReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, apiError("bad_request", "invalid payload"))
	}
	ownerID := apiOwnerID(c)
	token, rec, err := ctrl.model.CreateAPIToken(ownerID, nil, req.Name, req.Scope, req.ExpiresAt)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("db_error", "could not create token"))
	}
	return c.JSON(http.StatusCreated, createTokenResp{
		ID: rec.ID, Prefix: rec.TokenPrefix, Token: token,
	})
}

func (ctrl *controller) apiRevokeToken(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, apiError("bad_request", "invalid id"))
	}
	if err := ctrl.model.RevokeAPIToken(apiOwnerID(c), uint(id)); err != nil {
		return c.JSON(http.StatusInternalServerError, apiError("db_error", "could not revoke token"))
	}
	return c.NoContent(http.StatusNoContent)
}
