package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
)

type ProductPriceOracleRepository struct {
	oracleService service.OracleService
	logger        service.Logger
	redisClient   service.RedisClient
}

// NewOracleRepository creates a new OracleRepository with the given OracleService.
func NewProductPriceOracleRepository(oracleService service.OracleService, logger service.Logger, redisClient service.RedisClient) domain.ProductRepository {
	return &ProductPriceOracleRepository{
		oracleService: oracleService,
		logger:        logger,
		redisClient:   redisClient,
	}
}

// GetProductByUser calls PKG_TOKPED_PRODUCT.getProductByUser to get product list with status
// Example: select outerrcode,outerrmsg,kodevoucher,price,StatusProduct from table(PKG_TOKPED_PRODUCT.getProductByUser('USERTOKPEDDEV'))
func (r *ProductPriceOracleRepository) GetProductByUser(ctx context.Context, username string) (*[]domain.ProductPriceResponseDomain, error) {
	query := `SELECT outerrcode, outerrmsg, kodevoucher, price, StatusProduct FROM TABLE(PKG_TOKPED_PRODUCT.getProductByUser(:1))`
	r.logger.Info("Executing query", "query", query, "username", username)

	rows, err := r.oracleService.Query(ctx, query, username)
	if err != nil {
		r.logger.Error("Failed to execute query", "error", err, "query", query, "username", username)
		return nil, fmt.Errorf("failed to execute PKG_TOKPED_PRODUCT.getProductByUser: %w", err)
	}
	defer rows.Close()

	response := []domain.ProductPriceResponseDomain{}

	for rows.Next() {
		var outerRCode, outerRMsg, kodeVoucher, statusProduct sql.NullString
		var price sql.NullFloat64

		if err := rows.Scan(&outerRCode, &outerRMsg, &kodeVoucher, &price, &statusProduct); err != nil {
			return nil, fmt.Errorf("failed to scan result from PKG_TOKPED_PRODUCT.getProductByUser: %w", err)
		}

		product := domain.ProductPriceResponseDomain{}
		if outerRCode.Valid {
			product.OuterRCode = outerRCode.String
		}
		if outerRMsg.Valid {
			product.OuterRMsg = outerRMsg.String
		}
		if kodeVoucher.Valid {
			product.KodeVoucher = kodeVoucher.String
		}
		if price.Valid {
			product.Price = price.Float64
		}
		if statusProduct.Valid {
			product.Status = statusProduct.String
		}

		response = append(response, product)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Failed to iterate over results", "error", err, "query", query, "username", username)
		return nil, fmt.Errorf("error iterating over PKG_TOKPED_PRODUCT.getProductByUser results: %w", err)
	}

	r.logger.Info("Query executed successfully", "query", query, "username", username)
	return &response, nil
}

// GetPriceByUser implements ProductRepository interface
// Uses PKG_TOKPED_PRODUCT.GetKodeVoucher to get all product prices for user
func (r *ProductPriceOracleRepository) GetPriceByUser(ctx context.Context, username string, provider string) (*[]domain.ProductPriceResponseDomain, error) {
	// Prepare the call to the Oracle function that returns a cursor
	// For example: select outerrcode,outerrmsg,kodevoucher,price from table(PKG_TOKPED_PRODUCT.GetKodeVoucher('ALFA-DEV', 'PLN'))
	query := `SELECT outerrcode, outerrmsg, kodevoucher, price FROM TABLE(PKG_TOKPED_PRODUCT.GetKodeVoucher(:1, :2))`
	r.logger.Info("Executing query", "query", query, "username", username, "provider", provider)

	// Execute the query and get the result
	rows, err := r.oracleService.Query(ctx, query, username, provider)
	if err != nil {
		r.logger.Error("Failed to execute query", "error", err, "query", query, "username", username)
		return nil, fmt.Errorf("failed to execute PKG_TOKPED_PRODUCT.GetKodeVoucher: %w", err)
	}
	defer rows.Close()

	r.logger.Info("Query executed successfully", "query", query, "username", username)
	// Create response domain slice
	response := []domain.ProductPriceResponseDomain{}

	// Process multiple results - handle multiple rows
	for rows.Next() {
		var outerRCode, outerRMsg, kodeVoucher sql.NullString
		var price sql.NullFloat64

		err := rows.Scan(&outerRCode, &outerRMsg, &kodeVoucher, &price)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result from PKG_TOKPED_PRODUCT.GetKodeVoucher: %w", err)
		}

		// Create complete response object for each row
		productPrice := domain.ProductPriceResponseDomain{}

		// Convert sql.NullString and sql.NullFloat64 to actual values
		if outerRCode.Valid {
			productPrice.OuterRCode = outerRCode.String
		}
		if outerRMsg.Valid {
			productPrice.OuterRMsg = outerRMsg.String
		}
		if kodeVoucher.Valid {
			productPrice.KodeVoucher = kodeVoucher.String
		}
		if price.Valid {
			productPrice.Price = price.Float64
		}

		// Append complete product price to response
		response = append(response, productPrice)
	}

	// Check for any errors during iteration
	if err := rows.Err(); err != nil {
		r.logger.Error("Failed to iterate over results", "error", err, "query", query, "username", username)
		return nil, fmt.Errorf("error iterating over PKG_TOKPED_PRODUCT.GetKodeVoucher results: %w", err)
	}

	r.logger.Info("Query executed successfully", "query", query, "username", username)

	return &response, nil
}

// Implement ProductRepository interface methods

// GetPriceByUserAndProductCode implements ProductRepository interface
// Uses PKG_TOKPED_PRODUCT.GetKodeVoucherSelect to get specific product price
// For example: select outerrcode,outerrmsg,kodevoucher,price from table(PKG_TOKPED_PRODUCT.GetKodeVoucherSelect('ALFA-DEV','TS100'))
func (r *ProductPriceOracleRepository) GetPriceByUserAndProductCode(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
	// Prepare the call to the Oracle function that returns a cursor with specific product
	query := `SELECT outerrcode, outerrmsg, kodevoucher, price FROM TABLE(PKG_TOKPED_PRODUCT.GetKodeVoucherSelect(:1, :2))`
	r.logger.Info("Executing query", "query", query, "username", username, "productCode", productCode)
	// Execute the query and get the result
	rows, err := r.oracleService.Query(ctx, query, username, productCode)
	if err != nil {
		r.logger.Error("Failed to execute query", "error", err, "query", query, "username", username, "productCode", productCode)
		return nil, fmt.Errorf("failed to execute PKG_TOKPED_PRODUCT.GetKodeVoucherSelect: %w", err)
	}
	defer rows.Close()

	r.logger.Info("Query executed successfully", "query", query, "username", username, "productCode", productCode)

	// Create response domain
	response := &domain.ProductPriceResponseDomain{}

	// Process the result - expecting single row
	if rows.Next() {
		var outerRCode, outerRMsg, kodeVoucher sql.NullString
		var price sql.NullFloat64

		err := rows.Scan(&outerRCode, &outerRMsg, &kodeVoucher, &price)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result from PKG_TOKPED_PRODUCT.GetKodeVoucherSelect: %w", err)
		}

		// Convert sql.NullString and sql.NullFloat64 to actual values
		if outerRCode.Valid {
			response.OuterRCode = outerRCode.String
		}
		if outerRMsg.Valid {
			response.OuterRMsg = outerRMsg.String
		}
		if kodeVoucher.Valid {
			response.KodeVoucher = kodeVoucher.String
		}
		if price.Valid {
			response.Price = price.Float64
		}
	}

	// Check for any errors during iteration
	if err := rows.Err(); err != nil {
		r.logger.Error("Failed to iterate over results", "error", err, "query", query, "username", username, "productCode", productCode)
		return nil, fmt.Errorf("error iterating over PKG_TOKPED_PRODUCT.GetKodeVoucherSelect results: %w", err)
	}

	r.logger.Info("Query executed successfully", "query", query, "username", username, "productCode", productCode)

	return response, nil
}

// GetProductByUserAndProductCode calls PKG_TOKPED_PRODUCT.getProductByUserAndProductCode for specific product with status
// Example: select outerrcode,outerrmsg,kodevoucher,price,StatusProduct from table(PKG_TOKPED_PRODUCT.getProductByUserAndProductCode('USERTOKPEDDEV','PLNT1JT'))
func (r *ProductPriceOracleRepository) GetProductByUserAndProductCode(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
	query := `SELECT outerrcode, outerrmsg, kodevoucher, price, StatusProduct FROM TABLE(PKG_TOKPED_PRODUCT.getProductByUserAndProductCode(:1, :2))`
	r.logger.Info("Executing query", "query", query, "username", username, "productCode", productCode)

	rows, err := r.oracleService.Query(ctx, query, username, productCode)
	if err != nil {
		r.logger.Error("Failed to execute query", "error", err, "query", query, "username", username, "productCode", productCode)
		return nil, fmt.Errorf("failed to execute PKG_TOKPED_PRODUCT.getProductByUserAndProductCode: %w", err)
	}
	defer rows.Close()

	response := &domain.ProductPriceResponseDomain{}

	if rows.Next() {
		var outerRCode, outerRMsg, kodeVoucher, statusProduct sql.NullString
		var price sql.NullFloat64

		if err := rows.Scan(&outerRCode, &outerRMsg, &kodeVoucher, &price, &statusProduct); err != nil {
			return nil, fmt.Errorf("failed to scan result from PKG_TOKPED_PRODUCT.getProductByUserAndProductCode: %w", err)
		}

		if outerRCode.Valid {
			response.OuterRCode = outerRCode.String
		}
		if outerRMsg.Valid {
			response.OuterRMsg = outerRMsg.String
		}
		if kodeVoucher.Valid {
			response.KodeVoucher = kodeVoucher.String
		}
		if price.Valid {
			response.Price = price.Float64
		}
		if statusProduct.Valid {
			response.Status = statusProduct.String
		}
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Failed to iterate over results", "error", err, "query", query, "username", username, "productCode", productCode)
		return nil, fmt.Errorf("error iterating over PKG_TOKPED_PRODUCT.getProductByUserAndProductCode results: %w", err)
	}

	r.logger.Info("Query executed successfully", "query", query, "username", username, "productCode", productCode)
	return response, nil
}

// CacheProductPricesToRedis caches all product prices for a user to Redis
func (r *ProductPriceOracleRepository) CacheProductPricesToRedis(ctx context.Context, username string) error {
	r.logger.Info("Starting to cache product prices for user", "username", username)

	// Get product prices from Oracle
	provider := os.Getenv("PROVIDER_CODE_GET_PRICE") // PLN only
	prices, err := r.GetPriceByUser(ctx, username, provider)
	if err != nil {
		r.logger.Error("Failed to get product prices for caching", "error", err, "username", username)
		return fmt.Errorf("failed to get product prices for caching: %w", err)
	}

	// Cache each KodeVoucher as key and Price as value
	cachedCount := 0
	for _, price := range *prices {
		// Only cache if both KodeVoucher and Price are not empty
		if price.KodeVoucher != "" && price.Price > 0 {
			// Cache key format: product_price:{username}:{kodevoucher}
			cacheKey := fmt.Sprintf("product_price:%s", price.KodeVoucher)

			// Set cache with 24 hour expiration
			err = r.redisClient.Set(ctx, price.KodeVoucher, price.Price, 24*time.Hour).Err()
			if err != nil {
				r.logger.Error("Failed to cache product price to Redis", "error", err, "username", username, "kodeVoucher", price.KodeVoucher, "cacheKey", cacheKey)
				continue
			}
			cachedCount++
		} else {
			r.logger.Warn("Skipped caching product price - KodeVoucher or Price is empty",
				"username", username,
				"kodeVoucher", price.KodeVoucher,
				"price", price.Price)
		}
	}

	r.logger.Info("Successfully cached product prices to Redis", "username", username, "cachedCount", cachedCount, "totalCount", len(*prices))
	return nil
}

// CacheProductsWithStatusToRedis caches all products with status for a user to Redis
// Similar to CacheProductPricesToRedis but uses GetProductByUser to get status info
func (r *ProductPriceOracleRepository) CacheProductsWithStatusToRedis(ctx context.Context, username string) error {
	r.logger.Info("Starting to cache products with status for user", "username", username)

	// Get products with status from Oracle
	products, err := r.GetProductByUser(ctx, username)
	if err != nil {
		r.logger.Error("Failed to get products with status for caching", "error", err, "username", username)
		return fmt.Errorf("failed to get products with status for caching: %w", err)
	}

	// Cache each product as JSON with product info including status
	cachedCount := 0
	for _, product := range *products {
		// Only cache if KodeVoucher is not empty
		if product.KodeVoucher != "" {
			// Cache key format: product_with_status:{kodevoucher}
			cacheKey := fmt.Sprintf("product_with_status:%s", product.KodeVoucher)

			// Create a JSON representation of the product
			productJSON := fmt.Sprintf(`{"kodevoucher":"%s","price":%f,"status":"%s"}`,
				product.KodeVoucher, product.Price, product.Status)

			// Set cache with 24 hour expiration
			err = r.redisClient.Set(ctx, cacheKey, productJSON, 24*time.Hour).Err()
			if err != nil {
				r.logger.Error("Failed to cache product with status to Redis",
					"error", err,
					"username", username,
					"kodeVoucher", product.KodeVoucher,
					"cacheKey", cacheKey)
				continue
			}
			cachedCount++

			r.logger.Debug("Cached product with status",
				"kodeVoucher", product.KodeVoucher,
				"price", product.Price,
				"status", product.Status,
				"cacheKey", cacheKey)
		} else {
			r.logger.Warn("Skipped caching product with status - KodeVoucher is empty",
				"username", username,
				"status", product.Status)
		}
	}

	r.logger.Info("Successfully cached products with status to Redis",
		"username", username,
		"cachedCount", cachedCount,
		"totalCount", len(*products))
	return nil
}

// GetIpByUser implements ProductRepository interface
// Uses PKG_TOKPED_PRODUCT.GetIpByUser to get whitelisted IP for user
func (r *ProductPriceOracleRepository) GetIpByUser(ctx context.Context, username string) (*domain.WhitelistedIpResponseDomain, error) {
	query := `SELECT outerrcode, outerrmsg, outip FROM TABLE(PKG_TOKPED_PRODUCT.GetIpByUser(:1))`
	r.logger.Info("Executing GetIpByUser query", "query", query, "username", username)

	rows, err := r.oracleService.Query(ctx, query, username)
	if err != nil {
		r.logger.Error("Failed to execute GetIpByUser query", "error", err, "query", query, "username", username)
		return nil, fmt.Errorf("failed to execute PKG_TOKPED_PRODUCT.GetIpByUser: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		r.logger.Warn("No result returned from GetIpByUser query", "username", username)
		return nil, fmt.Errorf("no result from GetIpByUser for username: %s", username)
	}

	var outerRCode, outerRMsg, outIp sql.NullString
	err = rows.Scan(&outerRCode, &outerRMsg, &outIp)
	if err != nil {
		r.logger.Error("Failed to scan GetIpByUser result", "error", err, "username", username)
		return nil, fmt.Errorf("failed to scan result from PKG_TOKPED_PRODUCT.GetIpByUser: %w", err)
	}

	response := &domain.WhitelistedIpResponseDomain{}
	if outerRCode.Valid {
		response.OuterRCode = outerRCode.String
	}
	if outerRMsg.Valid {
		response.OuterRMsg = outerRMsg.String
	}
	if outIp.Valid {
		response.OutIp = outIp.String
	}

	r.logger.Info("GetIpByUser query executed successfully", "username", username, "outerRCode", response.OuterRCode, "outIp", response.OutIp)
	return response, nil
}

// GetCutOff implements ProductRepository interface
// Uses PKG_TOKPED_PRODUCT.GetCutOff to get cut-off configuration
func (r *ProductPriceOracleRepository) GetCutOff(ctx context.Context) (*domain.CutOffDataResponseDomain, error) {
	query := `SELECT outerrcode, outerrmsg, cut_off_time_start_tokopedia, cut_off_duration_tokopedia, cut_off_message_tokopedia, cut_off_time_start, cut_off_duration, cut_off_message FROM TABLE(PKG_TOKPED_PRODUCT.GetCutOff())`
	r.logger.Info("Executing GetCutOff query", "query", query)

	rows, err := r.oracleService.Query(ctx, query)
	if err != nil {
		r.logger.Error("Failed to execute GetCutOff query", "error", err, "query", query)
		return nil, fmt.Errorf("failed to execute PKG_TOKPED_PRODUCT.GetCutOff: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		r.logger.Warn("No result returned from GetCutOff query")
		return nil, fmt.Errorf("no result from GetCutOff")
	}

	var outErrCode, outErrMsg, cutOffTimeStartTokopedia, cutOffDurationTokopedia, cutOffMessageTokopedia sql.NullString
	var cutOffTimeStart, cutOffDuration, cutOffMessage sql.NullString

	err = rows.Scan(&outErrCode, &outErrMsg, &cutOffTimeStartTokopedia, &cutOffDurationTokopedia, &cutOffMessageTokopedia, &cutOffTimeStart, &cutOffDuration, &cutOffMessage)
	if err != nil {
		r.logger.Error("Failed to scan GetCutOff result", "error", err)
		return nil, fmt.Errorf("failed to scan result from PKG_TOKPED_PRODUCT.GetCutOff: %w", err)
	}

	response := &domain.CutOffDataResponseDomain{}
	if outErrCode.Valid {
		response.OutErrCode = outErrCode.String
	}
	if outErrMsg.Valid {
		response.OutErrMsg = outErrMsg.String
	}
	if cutOffTimeStartTokopedia.Valid {
		response.CutOffTimeStartTokopedia = cutOffTimeStartTokopedia.String
	}
	if cutOffDurationTokopedia.Valid {
		response.CutOffDurationTokopedia = cutOffDurationTokopedia.String
	}
	if cutOffMessageTokopedia.Valid {
		response.CutOffMessageTokopedia = cutOffMessageTokopedia.String
	}
	if cutOffTimeStart.Valid {
		response.CutOffTimeStart = cutOffTimeStart.String
	}
	if cutOffDuration.Valid {
		response.CutOffDuration = cutOffDuration.String
	}
	if cutOffMessage.Valid {
		response.CutOffMessage = cutOffMessage.String
	}

	r.logger.Info("GetCutOff query executed successfully", "outErrCode", response.OutErrCode)
	return response, nil
}
