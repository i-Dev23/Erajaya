package http

import (
	"pps-services-publisher-database/internal/model"
	"pps-services-publisher-database/internal/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
)

type TransactionController struct {
	Log     zerolog.Logger
	UseCase *usecase.TransactionUseCase
}

func NewTransactionController(useCase *usecase.TransactionUseCase, log zerolog.Logger) *TransactionController {
	return &TransactionController{
		Log:     log,
		UseCase: useCase,
	}
}

// SubmitCallback handles biller callback submissions.
// @Summary Submit callback dari biller
// @Description Menerima callback dari biller, kemudian di-publish ke RabbitMQ untuk diproses oleh consumer.
// @Tags Callback
// @Accept json
// @Produce json
// @Param X-API-Key header string true "API Key untuk autentikasi"
// @Param request body model.CallbackRequest true "Data callback"
// @Success 200 {object} model.WebResponse[model.CallbackResponse]
// @Failure 400 {object} model.WebResponse[string]
// @Failure 401 {object} model.WebResponse[string]
// @Failure 500 {object} model.WebResponse[string]
// @Router /api/callback/topup [post]
func (c *TransactionController) SubmitTopupCallback(ctx fiber.Ctx) error {
	// headers := ctx.GetReqHeaders()
	// headers := make(map[string][]string)
	// headers = map[string][]string{
	// 	// "X-Action": headers["X-Action"],
	// }
	headers := map[string][]string{
		"X-Type-Transaction": {"topup"},
	}

	request := new(model.CallbackRequest[model.TopupDataPayload])
	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warn().Msgf("Failed to parse callback body: %v", err)
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	response, err := c.UseCase.ProcessTopupDataCallback(ctx.Context(), headers, request)
	if err != nil {
		c.Log.Warn().Msgf("Failed to process callback: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(model.WebResponse[*model.CallbackResponse]{Data: response})
}

// SubmitCallback handles biller callback submissions.
// @Summary Submit callback dari biller
// @Description Menerima callback dari biller, kemudian di-publish ke RabbitMQ untuk diproses oleh consumer.
// @Tags Callback
// @Accept json
// @Produce json
// @Param X-API-Key header string true "API Key untuk autentikasi"
// @Param request body model.CallbackRequest true "Data callback"
// @Success 200 {object} model.WebResponse[model.CallbackResponse]
// @Failure 400 {object} model.WebResponse[string]
// @Failure 401 {object} model.WebResponse[string]
// @Failure 500 {object} model.WebResponse[string]
// @Router /api/callback/data [post]
func (c *TransactionController) SubmitDataCallback(ctx fiber.Ctx) error {
	headers := map[string][]string{
		"X-Type-Transaction": {"topup"},
	}

	request := new(model.CallbackRequest[model.TopupDataPayload])
	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warn().Msgf("Failed to parse callback body: %v", err)
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	response, err := c.UseCase.ProcessTopupDataCallback(ctx.Context(), headers, request)
	if err != nil {
		c.Log.Warn().Msgf("Failed to process callback: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(model.WebResponse[*model.CallbackResponse]{Data: response})
}

// HealthCheck checks the status of all dependent services.
// @Summary Health check
// @Description Mengecek status koneksi ke RabbitMQ
// @Tags Health
// @Produce json
// @Success 200 {object} model.WebResponse[model.HealthResponse]
// @Router /health [get]
func (c *TransactionController) HealthCheck(ctx fiber.Ctx) error {
	services := make(map[string]string)
	status := "healthy"

	if err := c.UseCase.PingRabbitMQ(); err != nil {
		services["rabbitmq"] = "unhealthy: " + err.Error()
		status = "degraded"
	} else {
		services["rabbitmq"] = "healthy"
	}

	response := &model.HealthResponse{
		Status:   status,
		Services: services,
	}

	return ctx.JSON(model.WebResponse[*model.HealthResponse]{Data: response})
}
