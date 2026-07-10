package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestServerMatchesEncodedObjectGUIDPathParameters(t *testing.T) {
	t.Parallel()
	server := New(Config{Debug: true})
	server.GET("/objects/:object_id", func(c *gin.Context) {
		c.String(http.StatusOK, c.Param("object_id"))
	})

	guid := "gid://axonhub/Request/42"
	w := httptest.NewRecorder()
	server.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/objects/"+url.PathEscape(guid), nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, guid, w.Body.String())
}
