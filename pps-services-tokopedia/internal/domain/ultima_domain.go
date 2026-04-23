package domain

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
)

// UltimaBaseResponseDomain represents the base response from Ultima API
type UltimaBaseResponseDomain struct {
	UrlUltima        string
	HttpStatus       string
	HttpStatusCode   int
	HttpResponseBody HttpResponseBody
}

// UltimaCheckIdPlnRequestDomain represents the request payload for checkIdPlnUltima API
type UltimaCheckIdPlnRequestDomain struct {
	IdPel string // Customer ID for PLN
	IdTrx string // Transaction ID
}

type HttpResponseBody struct {
	Msg     string         `json:"msg"`
	Produk  string         `json:"produk"`
	RC      string         `json:"rc"`
	ReffID  string         `json:"reffid"`
	SN      string         `json:"sn"`
	Status  StringOrNumber `json:"status"`
	Success bool           `json:"success"`
	Tujuan  string         `json:"tujuan"`
}

// GetStatusAsInt returns the status as int, converting from string if necessary
func (h *HttpResponseBody) GetStatusAsInt() (int, error) {
	// Normalize to int regardless of JSON type (string or number)
	s := string(h.Status)
	if s == "" {
		return 0, nil
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		// Attempt to parse as float then cast
		if f, ferr := strconv.ParseFloat(s, 64); ferr == nil {
			return int(f), nil
		}
		return 0, err
	}
	return i, nil
}

type PLNTransactionInquiry struct {
	Name        string
	MeterNumber string
	IDPelanggan string
	TarifDaya   string
}

// UltimaMappingError represents an error from Ultima with mapped response code
type UltimaMappingError struct {
	Message         string
	ResponseCode    string
	ResponseMessage string
}

func (e *UltimaMappingError) Error() string {
	return e.Message
}

// GetResponseCode returns the mapped response code
func (e *UltimaMappingError) GetResponseCode() string {
	return e.ResponseCode
}

// GetResponseMessage returns the mapped response message
func (e *UltimaMappingError) GetResponseMessage() string {
	return e.ResponseMessage
}

// UltimaService defines the interface for Ultima service
type UltimaService interface {
	CheckIdPlnUltima(ctx context.Context, req *UltimaCheckIdPlnRequestDomain) (*UltimaBaseResponseDomain, *PLNTransactionInquiry, error)
	Ping(ctx context.Context) error
}

// StringOrNumber is a helper type that accepts either a JSON string or number
// and stores its textual representation.
type StringOrNumber string

func (sn *StringOrNumber) UnmarshalJSON(b []byte) error {
	// If it's explicitly null
	if bytes.Equal(b, []byte("null")) {
		*sn = ""
		return nil
	}
	// If quoted, treat as string
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*sn = StringOrNumber(s)
		return nil
	}
	// Otherwise, try parse as JSON number using UseNumber
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return err
	}
	switch n := v.(type) {
	case json.Number:
		*sn = StringOrNumber(n.String())
		return nil
	case float64:
		*sn = StringOrNumber(strconv.FormatFloat(n, 'f', -1, 64))
		return nil
	default:
		// Fallback to raw bytes
		*sn = StringOrNumber(string(b))
		return nil
	}
}
