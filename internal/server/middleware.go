package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const maxRequestBodyBytes = 1 << 20

func limitRequestBody() gin.HandlerFunc {
	return func(requestContext *gin.Context) {
		requestContext.Request.Body = http.MaxBytesReader(requestContext.Writer, requestContext.Request.Body, maxRequestBodyBytes)
		requestContext.Next()
	}
}
