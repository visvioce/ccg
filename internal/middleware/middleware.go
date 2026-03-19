package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Api-Key")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func AuthMiddleware(host string, apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if host == "0.0.0.0" || host == "" {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		xApiKey := c.GetHeader("X-Api-Key")

		authKey := authHeader
		if authKey == "" {
			authKey = xApiKey
		}

		if authKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "APIKEY is missing",
			})
			return
		}

		token := authHeader
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		} else if xApiKey != "" {
			token = xApiKey
		} else {
			token = authKey
		}

		if token != apiKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key",
			})
			return
		}

		c.Next()
	}
}

func ModelParseMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasSuffix(c.Request.URL.Path, "/v1/messages") {
			c.Next()
			return
		}

		var body map[string]any
		if err := c.ShouldBindJSON(&body); err != nil {
			c.Next()
			return
		}

		if model, ok := body["model"].(string); ok && strings.Contains(model, ",") {
			parts := strings.SplitN(model, ",", 2)
			body["model"] = parts[1]
			c.Set("provider", parts[0])
		}

		if userId, ok := body["metadata"].(map[string]any); ok {
			if userIdStr, ok := userId["user_id"].(string); ok {
				if parts := strings.Split(userIdStr, "_session_"); len(parts) > 1 {
					c.Set("sessionId", parts[1])
				}
			}
		}

		c.Set("requestBody", body)
		c.Next()
	}
}
