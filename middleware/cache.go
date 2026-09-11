package middleware

import (
	"github.com/gin-gonic/gin"
)

func Cache() func(c *gin.Context) {
	return func(c *gin.Context) {
		if c.Request.RequestURI == "/" {
			c.Header("Cache-Control", "no-cache")
		} else {
			c.Header("Cache-Control", "max-age=604800") // one week
		}
		c.Header("Cache-Version", "c9d9d7336a16a322752aa7e684cd19670155cb819d6416d1dd32aa6b58fbd7fc")
		c.Next()
	}
}
