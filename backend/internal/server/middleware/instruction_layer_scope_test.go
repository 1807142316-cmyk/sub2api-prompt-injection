//go:build unit

package middleware

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInstructionLayerAuthenticatedScopeBothWrappers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{config.RunModeSimple, config.RunModeStandard} {
		for _, google := range []bool{false, true} {
			name := mode + "/standard"
			if google {
				name = mode + "/google"
			}
			t.Run(name, func(t *testing.T) {
				user := &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive, Balance: 10, Concurrency: 2}
				group := &service.Group{ID: 42, Name: "fixture", Status: service.StatusActive, Hydrated: true, Platform: service.PlatformOpenAI}
				key := &service.APIKey{ID: 100, UserID: user.ID, Key: "fixture-client-key", Status: service.StatusActive, User: user, Group: group, GroupID: &group.ID}
				repo := &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) { copy := *key; return &copy, nil }}
				cfg := &config.Config{RunMode: mode}
				svc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
				mw := gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg))
				if google {
					mw = APIKeyAuthWithSubscriptionGoogle(svc, nil, cfg)
				}
				router := gin.New()
				router.Use(mw)
				visited := false
				router.POST("/scope", func(c *gin.Context) {
					visited = true
					require.Equal(t, int64(7), c.Request.Context().Value(ctxkey.UserID))
					require.Equal(t, int64(100), c.Request.Context().Value(ctxkey.APIKeyID))
					require.Equal(t, int64(42), c.Request.Context().Value(ctxkey.AuthenticatedGroupID))
					setGroupContext(c, &service.Group{ID: 43, Name: "fallback", Status: service.StatusActive, Hydrated: true, Platform: service.PlatformOpenAI})
					require.Equal(t, int64(42), c.Request.Context().Value(ctxkey.AuthenticatedGroupID), "routing changes must not change the authenticated injection scope")
					c.Status(http.StatusOK)
				})
				request := httptest.NewRequest(http.MethodPost, "/scope", strings.NewReader(`{"user":"999","api_key_id":888,"group_id":999,"metadata":{"user_id":999,"group_id":999}}`))
				request.Header.Set("x-api-key", key.Key)
				request.Header.Set("x-goog-api-key", key.Key)
				request.Header.Set("X-User-ID", "999")
				request.Header.Set("X-API-Key-ID", "888")
				request.Header.Set("X-Group-ID", "999")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
				require.True(t, visited)
			})
		}
	}
}
