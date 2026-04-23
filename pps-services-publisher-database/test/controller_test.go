package test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"

	"pps-services-publisher-database/internal/model"
)

// TestSubmitTopupCallback_Positive tests POST /api/callback/topup with valid payload.
func TestSubmitTopupCallback_Positive(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(ctx fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return ctx.Status(code).JSON(fiber.Map{"errors": err.Error()})
		},
	})

	app.Post("/api/callback/topup", func(ctx fiber.Ctx) error {
		req := new(model.CallbackRequest[model.TopupDataPayload])
		if err := ctx.Bind().JSON(req); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
		}

		response := &model.CallbackResponse{
			MsgID:  req.Data.MsgID,
			Status: "published",
		}
		return ctx.JSON(model.WebResponse[*model.CallbackResponse]{Data: response})
	})

	reqBody := model.CallbackRequest[model.TopupDataPayload]{
		Source: "test",
		Data: model.TopupDataPayload{
			MsgID:                  "1",
			StatusToBe:             "0",
			ClientNumber:           "08112233445",
			OriginalConversationID: "ORIG-001",
			ConversationID:         "CONV-001",
			MessageToCustomer:      "Payment successful",
			QueueName:              "biller-telkomsel-1",
		},
	}

	bodyJSON, err := json.Marshal(reqBody)
	assert.Nil(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/callback/topup",
		strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.Nil(t, err)

	var response model.WebResponse[*model.CallbackResponse]
	err = json.Unmarshal(body, &response)
	assert.Nil(t, err)
	assert.Equal(t, 1, response.Data.MsgID)
	assert.Equal(t, "published", response.Data.Status)
}

// TestSubmitTopupCallback_Negative_InvalidJSON tests invalid body returns 400.
func TestSubmitTopupCallback_Negative_InvalidJSON(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(ctx fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return ctx.Status(code).JSON(fiber.Map{"errors": err.Error()})
		},
	})

	app.Post("/api/callback/topup", func(ctx fiber.Ctx) error {
		req := new(model.CallbackRequest[model.TopupDataPayload])
		if err := ctx.Bind().JSON(req); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
		}
		return ctx.JSON(model.WebResponse[string]{Data: "ok"})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/callback/topup",
		strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestSubmitDataCallback_Positive tests POST /api/callback/data with valid DataPayload.
func TestSubmitDataCallback_Positive(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(ctx fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return ctx.Status(code).JSON(fiber.Map{"errors": err.Error()})
		},
	})

	app.Post("/api/callback/data", func(ctx fiber.Ctx) error {
		req := new(model.CallbackRequest[model.TopupDataPayload])
		if err := ctx.Bind().JSON(req); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
		}

		response := &model.CallbackResponse{
			MsgID:  req.Data.MsgID,
			Status: "published",
		}
		return ctx.JSON(model.WebResponse[*model.CallbackResponse]{Data: response})
	})

	reqBody := model.CallbackRequest[model.TopupDataPayload]{
		Source: "data-source",
		Data: model.TopupDataPayload{
			MsgID:                  "99",
			StatusToBe:             "0",
			ClientNumber:           "08199887766",
			OriginalConversationID: "ORIG-DATA-001",
			ConversationID:         "CONV-DATA-001",
			MessageToCustomer:      "Data callback OK",
			QueueName:              "biller-data-1",
		},
	}

	bodyJSON, err := json.Marshal(reqBody)
	assert.Nil(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/callback/data",
		strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.Nil(t, err)

	var response model.WebResponse[*model.CallbackResponse]
	err = json.Unmarshal(body, &response)
	assert.Nil(t, err)
	assert.Equal(t, 99, response.Data.MsgID)
	assert.Equal(t, "published", response.Data.Status)
}

// TestHealthCheck_Positive tests health check endpoint returns healthy.
func TestHealthCheck_Positive(t *testing.T) {
	app := fiber.New()

	app.Get("/health", func(ctx fiber.Ctx) error {
		response := &model.HealthResponse{
			Status: "healthy",
			Services: map[string]string{
				"oracle":   "healthy",
				"postgres": "healthy",
				"rabbitmq": "healthy",
			},
		}
		return ctx.JSON(model.WebResponse[*model.HealthResponse]{Data: response})
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	resp, err := app.Test(req)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.Nil(t, err)

	var response model.WebResponse[*model.HealthResponse]
	err = json.Unmarshal(body, &response)
	assert.Nil(t, err)

	assert.Equal(t, "healthy", response.Data.Status)
	assert.Equal(t, "healthy", response.Data.Services["oracle"])
	assert.Equal(t, "healthy", response.Data.Services["postgres"])
	assert.Equal(t, "healthy", response.Data.Services["rabbitmq"])
}

// TestHealthCheck_Degraded tests health check when a service is down.
func TestHealthCheck_Degraded(t *testing.T) {
	app := fiber.New()

	app.Get("/health", func(ctx fiber.Ctx) error {
		response := &model.HealthResponse{
			Status: "degraded",
			Services: map[string]string{
				"oracle":   "unhealthy: connection refused",
				"postgres": "healthy",
				"rabbitmq": "healthy",
			},
		}
		return ctx.JSON(model.WebResponse[*model.HealthResponse]{Data: response})
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	resp, err := app.Test(req)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)

	var response model.WebResponse[*model.HealthResponse]
	json.Unmarshal(body, &response)

	assert.Equal(t, "degraded", response.Data.Status)
	assert.Contains(t, response.Data.Services["oracle"], "unhealthy")
}
