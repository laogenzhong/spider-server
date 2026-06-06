package gateway

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func (s *GatewayServer) supportHandler(c *gin.Context) {
	servePublicHTML(c, "support.html")
}

func (s *GatewayServer) indexHandler(c *gin.Context) {
	servePublicHTML(c, "index.html")
}

func (s *GatewayServer) privacyHandler(c *gin.Context) {
	servePublicHTML(c, "privacy.html")
}

func servePublicHTML(c *gin.Context, filename string) {
	path, err := resolvePublicHTML(filename)
	if err != nil {
		c.String(http.StatusNotFound, "public page not found: %s", filename)
		return
	}

	body, err := os.ReadFile(path)
	if err != nil {
		c.String(http.StatusInternalServerError, "read public page failed")
		return
	}

	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", body)
}

func resolvePublicHTML(filename string) (string, error) {
	candidates := []string{
		filepath.Join("public", filename),
	}

	if exePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exePath), "public", filename))
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("public html file not found: %s", filename)
}
