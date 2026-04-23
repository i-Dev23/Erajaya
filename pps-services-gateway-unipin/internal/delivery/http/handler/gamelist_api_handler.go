package handler

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"

	contractsvc "pps-services-gateway-unipin/internal/domain/contract/service"
	"pps-services-gateway-unipin/internal/domain/contract/repository"
)

// GameListAPIHandler handles POST /api/v1/game-list.
type GameListAPIHandler struct {
	repo   repository.GameAPIRepository
	logger contractsvc.Logger
}

// NewGameListAPIHandler creates a new GameListAPIHandler.
func NewGameListAPIHandler(repo repository.GameAPIRepository, logger contractsvc.Logger) *GameListAPIHandler {
	return &GameListAPIHandler{repo: repo, logger: logger}
}

type gameListRequest struct {
	User      string `json:"user"`
	Timestamp string `json:"timestamp"`
	Signature string `json:"signature"`
}

type gameListResponse struct {
	Status  string        `json:"status"`
	Message string        `json:"message"`
	Data    []productItem `json:"data,omitempty"`
}

type productItem struct {
	Product     string            `json:"product"`
	ProductDesc string            `json:"product_desc"`
	Fields      []json.RawMessage `json:"fields"`
}

// HandleGameList handles POST /api/v1/game-list.
func (h *GameListAPIHandler) HandleGameList(c *fiber.Ctx) error {
	var req gameListRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(gameListResponse{
			Status: "1", Message: "invalid request body",
		})
	}

	if req.User == "" || req.Signature == "" {
		return c.Status(fiber.StatusBadRequest).JSON(gameListResponse{
			Status: "1", Message: "user and signature are required",
		})
	}

	// Validate signature via Oracle SP
	outError, outMessage, err := h.repo.ValidateSignature(c.UserContext(), req.User, req.Signature)
	if err != nil {
		h.logger.Error("validate signature failed", "user", req.User, "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(gameListResponse{
			Status: "1", Message: "internal server error",
		})
	}
	if outError != 0 {
		h.logger.Warn("signature validation failed", "user", req.User, "outError", outError, "outMessage", outMessage)
		return c.Status(fiber.StatusUnauthorized).JSON(gameListResponse{
			Status: "1", Message: outMessage,
		})
	}

	// Get game list from Oracle
	rows, err := h.repo.GetGameList(c.UserContext())
	if err != nil {
		h.logger.Error("get game list failed", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(gameListResponse{
			Status: "1", Message: "failed to retrieve game list",
		})
	}

	// Compose rows into grouped product array
	// Each row has product_code, product_desc, fields (single JSON string per row)
	// Group by product_code, collect fields into array
	productMap := make(map[string]*productItem)
	var productOrder []string

	for _, row := range rows {
		item, exists := productMap[row.ProductCode]
		if !exists {
			item = &productItem{
				Product:     row.ProductCode,
				ProductDesc: row.ProductDesc,
				Fields:      []json.RawMessage{},
			}
			productMap[row.ProductCode] = item
			productOrder = append(productOrder, row.ProductCode)
		}
		if row.Fields != "" {
			item.Fields = append(item.Fields, json.RawMessage(row.Fields))
		}
	}

	data := make([]productItem, 0, len(productOrder))
	for _, code := range productOrder {
		data = append(data, *productMap[code])
	}

	return c.JSON(gameListResponse{
		Status:  "0",
		Message: "Successfully",
		Data:    data,
	})
}
