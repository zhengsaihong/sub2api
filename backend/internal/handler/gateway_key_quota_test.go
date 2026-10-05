package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type keyQuotaAccountRepo struct {
	service.AccountRepository
	accounts []service.Account
}

func (r *keyQuotaAccountRepo) ListByGroup(context.Context, int64) ([]service.Account, error) {
	return r.accounts, nil
}

func newKeyQuotaGatewayService(repo service.AccountRepository) *service.GatewayService {
	return service.NewGatewayService(
		repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

func TestGatewayHandlerKeyQuotaInfoUsesAuthenticatedGroup(t *testing.T) {
	groupID := int64(7)
	cooldownUntil := time.Now().Add(time.Hour)
	apiKey := &service.APIKey{GroupID: &groupID, Key: "sk-quota-test"}
	repo := &keyQuotaAccountRepo{accounts: []service.Account{
		{Status: service.StatusActive, Schedulable: true, Extra: map[string]any{"codex_5h_used_percent": 25}},
		{Status: service.StatusActive, Schedulable: true, RateLimitResetAt: &cooldownUntil, Extra: map[string]any{"codex_5h_used_percent": 50}},
	}}
	h := &GatewayHandler{gatewayService: newKeyQuotaGatewayService(repo)}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/sub2api/quota", nil)
	c.Set(string(servermiddleware.ContextKeyAPIKey), apiKey)
	h.KeyQuotaInfo(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	var body service.GroupQuotaSummary
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, groupID, body.GroupID)
	require.Equal(t, 2, body.AccountCount)
	require.Equal(t, 1, body.CooldownAccountCount)
	require.NotNil(t, body.Remaining5hPercent)
	require.InDelta(t, 62.5, *body.Remaining5hPercent, 0.001)
	require.NotContains(t, w.Body.String(), apiKey.Key)
}

func TestGatewayHandlerKeyQuotaInfoRejectsUnassignedKey(t *testing.T) {
	h := &GatewayHandler{gatewayService: newKeyQuotaGatewayService(nil)}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/sub2api/quota", nil)
	c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{})
	h.KeyQuotaInfo(c)

	require.Equal(t, http.StatusForbidden, w.Code)
}
