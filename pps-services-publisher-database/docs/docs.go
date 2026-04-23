// Package docs berisi dokumentasi Swagger yang di-generate oleh swag.
// File ini di-generate otomatis oleh perintah: swag init -g cmd/web/main.go
// JANGAN edit file ini secara manual.
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {
            "name": "PPS Team"
        },
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/api/transactions": {
            "post": {
                "security": [{"ApiKeyAuth": []}],
                "description": "Menerima satu atau banyak hasil transaksi dari multi-biller, kemudian di-publish ke RabbitMQ berdasarkan queue_name masing-masing.",
                "consumes": ["application/json"],
                "produces": ["application/json"],
                "tags": ["Transactions"],
                "summary": "Submit batch transaksi dari biller",
                "parameters": [
                    {
                        "type": "string",
                        "description": "API Key untuk autentikasi",
                        "name": "X-API-Key",
                        "in": "header",
                        "required": true
                    },
                    {
                        "description": "Batch transaksi",
                        "name": "request",
                        "in": "body",
                        "required": true,
                        "schema": {"$ref": "#/definitions/model.BatchTransactionRequest"}
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {"$ref": "#/definitions/model.WebResponse-model_BatchTransactionResponse"}
                    },
                    "400": {"description": "Bad Request", "schema": {"$ref": "#/definitions/model.WebResponse-string"}},
                    "401": {"description": "Unauthorized", "schema": {"$ref": "#/definitions/model.WebResponse-string"}},
                    "500": {"description": "Internal Server Error", "schema": {"$ref": "#/definitions/model.WebResponse-string"}}
                }
            }
        },
        "/api/transactions/callbacks": {
            "post": {
                "security": [{"ApiKeyAuth": []}],
                "description": "Menerima callback dari biller, melengkapi data dari PostgreSQL, kemudian di-publish ke RabbitMQ untuk diproses oleh consumer.",
                "consumes": ["application/json"],
                "produces": ["application/json"],
                "tags": ["Transactions"],
                "summary": "Submit callback dari biller",
                "parameters": [
                    {
                        "type": "string",
                        "description": "API Key untuk autentikasi",
                        "name": "X-API-Key",
                        "in": "header",
                        "required": true
                    },
                    {
                        "description": "Data callback",
                        "name": "request",
                        "in": "body",
                        "required": true,
                        "schema": {"$ref": "#/definitions/model.CallbackRequest"}
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {"$ref": "#/definitions/model.WebResponse-model_CallbackResponse"}
                    },
                    "400": {"description": "Bad Request", "schema": {"$ref": "#/definitions/model.WebResponse-string"}},
                    "401": {"description": "Unauthorized", "schema": {"$ref": "#/definitions/model.WebResponse-string"}},
                    "404": {"description": "Not Found", "schema": {"$ref": "#/definitions/model.WebResponse-string"}},
                    "500": {"description": "Internal Server Error", "schema": {"$ref": "#/definitions/model.WebResponse-string"}}
                }
            }
        },
        "/health": {
            "get": {
                "description": "Mengecek status koneksi ke Oracle, PostgreSQL, dan RabbitMQ",
                "produces": ["application/json"],
                "tags": ["Health"],
                "summary": "Health check",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {"$ref": "#/definitions/model.WebResponse-model_HealthResponse"}
                    }
                }
            }
        }
    },
    "definitions": {
        "model.TransactionRequest": {
            "type": "object",
            "required": ["id", "queue_name", "rc", "product", "client_number", "message"],
            "properties": {
                "id": {"type": "string", "maxLength": 100, "minLength": 1},
                "queue_name": {"type": "string", "maxLength": 100, "minLength": 1},
                "rc": {"type": "string", "maxLength": 10, "minLength": 1},
                "product": {"type": "string", "maxLength": 100, "minLength": 1},
                "client_number": {"type": "string", "maxLength": 100, "minLength": 1},
                "message": {"type": "string", "maxLength": 500, "minLength": 1},
                "serial_number": {"type": "string", "maxLength": 100},
                "nominal": {"type": "string", "maxLength": 50},
                "additional_message": {"type": "string", "maxLength": 500}
            }
        },
        "model.BatchTransactionRequest": {
            "type": "object",
            "required": ["transactions"],
            "properties": {
                "transactions": {
                    "type": "array",
                    "minItems": 1,
                    "items": {"$ref": "#/definitions/model.TransactionRequest"}
                }
            }
        },
        "model.CallbackRequest": {
            "type": "object",
            "required": ["id", "conversation_id", "status_to_be"],
            "properties": {
                "id": {"type": "string", "maxLength": 100, "minLength": 1},
                "conversation_id": {"type": "string", "maxLength": 100, "minLength": 1},
                "original_conversation_id": {"type": "string", "maxLength": 100},
                "status_to_be": {"type": "string", "maxLength": 20, "minLength": 1},
                "message_to_customer": {"type": "string", "maxLength": 500},
                "additional_message": {"type": "string", "maxLength": 500}
            }
        },
        "model.TransactionResponse": {
            "type": "object",
            "properties": {
                "id": {"type": "string"},
                "queue_name": {"type": "string"},
                "status": {"type": "string"}
            }
        },
        "model.BatchTransactionResponse": {
            "type": "object",
            "properties": {
                "total": {"type": "integer"},
                "success": {"type": "integer"},
                "failed": {"type": "integer"},
                "results": {"type": "array", "items": {"$ref": "#/definitions/model.TransactionResponse"}}
            }
        },
        "model.CallbackResponse": {
            "type": "object",
            "properties": {
                "id": {"type": "string"},
                "status": {"type": "string"}
            }
        },
        "model.HealthResponse": {
            "type": "object",
            "properties": {
                "status": {"type": "string"},
                "services": {"type": "object", "additionalProperties": {"type": "string"}}
            }
        },
        "model.WebResponse-model_BatchTransactionResponse": {
            "type": "object",
            "properties": {
                "data": {"$ref": "#/definitions/model.BatchTransactionResponse"},
                "errors": {"type": "string"}
            }
        },
        "model.WebResponse-model_CallbackResponse": {
            "type": "object",
            "properties": {
                "data": {"$ref": "#/definitions/model.CallbackResponse"},
                "errors": {"type": "string"}
            }
        },
        "model.WebResponse-model_HealthResponse": {
            "type": "object",
            "properties": {
                "data": {"$ref": "#/definitions/model.HealthResponse"},
                "errors": {"type": "string"}
            }
        },
        "model.WebResponse-string": {
            "type": "object",
            "properties": {
                "data": {"type": "string"},
                "errors": {"type": "string"}
            }
        }
    },
    "securityDefinitions": {
        "ApiKeyAuth": {
            "type": "apiKey",
            "name": "X-API-Key",
            "in": "header"
        }
    }
}`

// SwaggerInfo menyimpan metadata Swagger API.
var SwaggerInfo = &swag.Spec{
	Version:          "1.0.0",
	Host:             "localhost:3000",
	BasePath:         "/",
	Schemes:          []string{},
	Title:            "PPS Services Database API",
	Description:      "REST API untuk menerima transaksi dari biller, publish ke RabbitMQ, dan simpan ke Oracle via stored procedure.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
