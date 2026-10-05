package handler

import (
	"net/http"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// KeyQuotaInfo returns the aggregate cached quota for the authenticated API key's group.
// GET /v1/sub2api/quota
func (h *GatewayHandler) KeyQuotaInfo(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	if apiKey.GroupID == nil {
		h.errorResponse(c, http.StatusForbidden, "permission_error", "API key is not assigned to a group")
		return
	}
	if h.gatewayService == nil {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "Group quota is unavailable")
		return
	}

	quota, err := h.gatewayService.GetGroupQuotaSummary(c.Request.Context(), *apiKey.GroupID)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "Group quota is unavailable")
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, quota)
}
