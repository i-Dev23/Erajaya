package repository

import "context"

// GameProductRow represents a single product row from Oracle game list query.
type GameProductRow struct {
	ProductCode string
	ProductDesc string
	Fields      string // single JSON string per row, e.g. {"name":"userid","type":"string"}
}

// GameAPIRepository defines the interface for game list API operations against Oracle.
type GameAPIRepository interface {
	// ValidateSignature calls MSG.PKG_UNIPIN.validateSignatureGameList.
	// Returns outError and outMessage. outError != 0 means validation failed.
	ValidateSignature(ctx context.Context, user, signature string) (int, string, error)

	// GetGameList calls the Oracle function to select product_code, product_desc, fields.
	// Returns one row per field. SP name TBA — currently uses placeholder.
	GetGameList(ctx context.Context) ([]GameProductRow, error)
}
