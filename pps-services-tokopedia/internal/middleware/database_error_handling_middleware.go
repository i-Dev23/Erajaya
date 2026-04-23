package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"pps-services-tokopedia/internal/dto"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// DatabaseErrorHandlingMiddleware catches database/service connection errors
// and returns response code 62 (Server Error)
func DatabaseErrorHandlingMiddleware(logger service.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Proceed with request
		if err := c.Next(); err != nil {
			// Check if error is a DatabaseError or a connection error
			isDatabaseError := false
			var dbErr *utils.DatabaseError

			// Check if it's explicitly a DatabaseError type
			if errors.As(err, &dbErr) {
				isDatabaseError = true
				logger.Error("Database/Service connection error detected",
					"error", err.Error(),
					"service", dbErr.ServiceName,
					"operation", dbErr.OperationType,
					"path", c.Path(),
					"method", c.Method())
			} else if utils.IsConnectionError(err) {
				// Check if it's a connection error pattern
				isDatabaseError = true
				logger.Error("Database/Service connection error detected",
					"error", err.Error(),
					"path", c.Path(),
					"method", c.Method())
			}

			if isDatabaseError {
				// Get response code 62 for server error
				responseCode, _ := utils.GetDatabaseErrorResponseCode()

				// Create error response based on the path
				path := c.Path()
				var plainBody interface{}

				// Remove route prefix from path
				path = strings.TrimPrefix(path, "/api/v1/")
				path = strings.TrimPrefix(path, "/auth/")

				switch path {
				case "token":
					plainBody = dto.TokenResponseDto{
						ResponseCode: responseCode.Code,
						Message:      responseCode.Message,
						Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
					}
				case "inquiry":
					plainBody = dto.InquiryResponseDto{
						ResponseCode: responseCode.Code,
						Message:      responseCode.Message,
						Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
					}
				case "payment":
					plainBody = dto.PaymentResponseDto{
						ResponseCode: responseCode.Code,
						Message:      responseCode.Message,
						Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
					}
				case "check-status":
					plainBody = dto.CheckStatusResponseDto{
						ResponseCode: responseCode.Code,
						Message:      responseCode.Message,
						Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
					}
				default:
					plainBody = dto.BaseTokopediaResponseDto{
						ResponseCode: responseCode.Code,
						Message:      responseCode.Message,
						Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
					}
				}

				// Marshal response
				plainBodyBytes, marshalErr := json.Marshal(plainBody)
				if marshalErr != nil {
					logger.Error("Failed to marshal database error response", "error", marshalErr)
					return c.Status(http.StatusInternalServerError).JSON(map[string]string{
						"response_code": "62",
						"message":       "Server error",
					})
				}

				// Set response body for encryption/signing
				c.Response().SetBody(plainBodyBytes)

				return nil // Let encryption middleware handle the response
			}

			// Return original error if not a connection error
			return err
		}

		return nil
	}
}
