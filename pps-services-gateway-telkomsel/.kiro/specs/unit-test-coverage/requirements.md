# Requirements Document

## Introduction

This feature establishes comprehensive unit test coverage across the pps-services-gateway-telkomsel Go service codebase. The goal is to achieve a minimum of 90% code coverage for all packages by introducing unit tests with mocks, table-driven tests, and httptest-based API client tests. The service integrates with Telkomsel ESB Modern Channel APIs, RabbitMQ message queues, and PostgreSQL databases. Each package requires targeted test strategies: pure function testing for helpers and validators, mock-based testing for infrastructure adapters, httptest server testing for the HTTP client, and fiber-based request testing for the callback handler.

## Glossary

- **Test_Suite**: A collection of Go test functions in a `_test.go` file within a specific package
- **Config_Loader**: The `internal/config` package responsible for reading environment variables and returning typed configuration structs (`Config`, `CallbackServerConfig`, `TelkomselConfig`)
- **Telkomsel_Client**: The `pkg/telkomsel.Client` struct that makes HTTP calls to Telkomsel ESB Modern Channel API endpoints (InitiateRegularRecharge, BrowseOffer, OrderDealer, CheckOrderStatus)
- **Callback_Handler**: The `internal/handler.CallbackHandler` struct that processes HTTP GET callback requests from Telkomsel via the `/callback/ext` route
- **HTTP_Server**: The `internal/http.Server` struct that manages the gofiber HTTP server lifecycle including routing and graceful shutdown
- **Consumer_Service**: The `internal/infrastructure/rabbitmq.ConsumerServiceImpl` struct that consumes RabbitMQ messages and dispatches Telkomsel API calls based on product type
- **MQ_Publisher**: The `internal/infrastructure/mqpublisher.AMQPPublisher` struct that publishes JSON messages to RabbitMQ queues
- **API_Log_Repository**: The `internal/infrastructure/postgres.APILogRepositoryImpl` struct that persists API call logs to PostgreSQL
- **API_Logger_Adapter**: The `internal/infrastructure/postgres.APILoggerAdapter` struct that adapts `APILogRepository` to the `telkomsel.APILogger` interface
- **Transaction_Logger**: The `internal/infrastructure/postgres.PostgresTransactionLogger` struct that manages transaction and response records in PostgreSQL
- **Element1_Encryptor**: The `pkg/telkomsel.EncryptElement1` function that encrypts a PIN using AES-128-CBC with a base64-encoded key
- **MSISDN_Normalizer**: The `pkg/telkomsel.normalizeMSISDN` function that converts various MSISDN formats to the 13-character `62`-prefixed format
- **Consume_Wrapper**: Package-level functions (`InitiateRegularRechargeOnConsume`, `BrowseOfferOnConsume`, `OrderDealerOnConsume`, `CheckOrderStatusOnConsume`) that read environment variables and delegate to Telkomsel_Client methods
- **Consume_Payload**: The `internal/infrastructure/rabbitmq.consumePayload` struct that deserializes flexible JSON message bodies from RabbitMQ
- **Provider_Publish_Message**: The `internal/infrastructure/mqpublisher.ProviderPublishMessage` struct wrapping downstream publish data with `source = "PROVIDER"`
- **Mock**: A test double implementing a contract interface to isolate the unit under test from external dependencies (database, message queue, HTTP)
- **Coverage_Target**: The minimum percentage of executable lines exercised by the Test_Suite, set at 90%

## Requirements

### Requirement 1: Configuration Loader Unit Tests

**User Story:** As a developer, I want unit tests for the configuration loading functions, so that I can verify environment variable parsing, defaults, and validation without relying on a running environment.

#### Acceptance Criteria

1. WHEN all required environment variables are set with valid values, THE Config_Loader `Load` function SHALL return a `Config` struct with fields matching the environment values
2. WHEN the `RABBITMQ_URL` environment variable is empty, THE Config_Loader `Load` function SHALL return an error containing "RABBITMQ_URL is required"
3. WHEN the `QUEUE_NAME_PROVIDER` environment variable is empty and `QUEUE_NAME` is set, THE Config_Loader `Load` function SHALL use the `QUEUE_NAME` value as the queue name
4. WHEN the `CONSUMER_TAG` environment variable is empty, THE Config_Loader `Load` function SHALL default to "pps-services-gateway-telkomsel-consumer"
5. WHEN the `CALLBACK_PORT` environment variable is empty, THE Config_Loader `LoadCallbackServer` function SHALL default to port 8080
6. WHEN the `CALLBACK_PORT` environment variable contains a non-numeric value, THE Config_Loader `LoadCallbackServer` function SHALL return an error
7. WHEN all required Telkomsel environment variables are set, THE Config_Loader `LoadTelkomsel` function SHALL return a `TelkomselConfig` struct with correct field values
8. WHEN any required Telkomsel environment variable is missing, THE Config_Loader `LoadTelkomsel` function SHALL return an error identifying the missing variable
9. WHEN the `TIMEOUT` environment variable is set to a valid positive integer, THE Config_Loader `LoadTelkomsel` function SHALL use that value as the timeout in seconds
10. WHEN the `TIMEOUT` environment variable is set to zero or a negative number, THE Config_Loader `LoadTelkomsel` function SHALL return an error

### Requirement 2: Telkomsel Client Construction and Validation Unit Tests

**User Story:** As a developer, I want unit tests for the Telkomsel client constructor and request validators, so that I can verify input validation logic catches all invalid inputs before making API calls.

#### Acceptance Criteria

1. WHEN valid parameters are provided, THE Telkomsel_Client `NewClient` function SHALL return a non-nil Client and nil error
2. WHEN any required parameter (baseURL, channelID, secretKey, apiKey) is empty, THE Telkomsel_Client `NewClient` function SHALL return an error identifying the missing parameter
3. WHEN timeout is zero or negative, THE Telkomsel_Client `NewClient` function SHALL return an error stating "timeout must be greater than zero"
4. WHEN a valid `InitiateRegularRechargeRequest` is provided, THE Telkomsel_Client `validateInitiateRegularRechargeRequest` validator SHALL return nil
5. WHEN `transaction_id` exceeds 25 characters in an `InitiateRegularRechargeRequest`, THE Telkomsel_Client validator SHALL return an error stating "transaction.transaction_id must be at most 25 characters"
6. WHEN `service_id` does not start with "62" or is not 13 characters, THE Telkomsel_Client validator SHALL return an error describing the constraint violation
7. WHEN `stock_type` is not "FIXED" or "BULK", THE Telkomsel_Client validator SHALL return an error listing the valid values
8. WHEN a valid `OrderDealerRequest` is provided, THE Telkomsel_Client `validateOrderDealerRequest` validator SHALL return nil
9. WHEN a valid `BrowseOfferRequest` is provided, THE Telkomsel_Client `validateBrowseOfferRequest` validator SHALL return nil
10. WHEN a valid `CheckOrderStatusRequest` is provided, THE Telkomsel_Client `validateCheckOrderStatusRequest` validator SHALL return nil
11. WHEN any required field is missing in `OrderDealerRequest`, `BrowseOfferRequest`, or `CheckOrderStatusRequest`, THE Telkomsel_Client validator SHALL return an error identifying the missing field

### Requirement 3: Telkomsel Client API Method Unit Tests

**User Story:** As a developer, I want unit tests for each Telkomsel client API method using httptest mock servers, so that I can verify correct HTTP request construction, response parsing, error classification, and retry behavior.

#### Acceptance Criteria

1. WHEN the mock server returns HTTP 200 with a valid JSON body containing `status_code` "00000", THE Telkomsel_Client `InitiateRegularRecharge` method SHALL return a parsed response and nil error
2. WHEN the mock server returns HTTP 200 with a `status_code` other than "00000", THE Telkomsel_Client `InitiateRegularRecharge` method SHALL return a `BusinessError` with the status code and description from the response
3. WHEN the mock server returns HTTP 500, THE Telkomsel_Client `InitiateRegularRecharge` method SHALL return a `TechnicalError` with the status code
4. WHEN the mock server returns HTTP 200 with valid JSON for BrowseOffer, THE Telkomsel_Client `BrowseOffer` method SHALL return a parsed `BrowseOfferResponse` with product details and nil error
5. WHEN the mock server returns HTTP 400 with a JSON body containing a non-"00000" `status_code`, THE Telkomsel_Client `BrowseOffer` method SHALL return a `BusinessError` and the parsed response
6. WHEN the mock server returns HTTP 200 with valid JSON for OrderDealer, THE Telkomsel_Client `OrderDealer` method SHALL return a parsed `OrderDealerResponse` and nil error
7. WHEN the mock server returns HTTP 200 with valid JSON for CheckOrderStatus, THE Telkomsel_Client `CheckOrderStatus` method SHALL return a parsed `CheckOrderStatusResponse` and nil error
8. WHEN the mock server returns HTTP 400 with a JSON body containing a non-"00000" `status_code` for CheckOrderStatus, THE Telkomsel_Client `CheckOrderStatus` method SHALL return a `BusinessError` and the parsed response
9. WHEN the mock server consistently returns HTTP 500, THE Telkomsel_Client SHALL retry up to `maxRetries` times before returning the final `TechnicalError`
10. WHEN a `BusinessError` is returned on the first attempt, THE Telkomsel_Client SHALL return the error immediately without retrying

### Requirement 4: Element1 Encryption Unit Tests

**User Story:** As a developer, I want unit tests for the AES-128-CBC Element1 encryption function, so that I can verify correct encryption output and error handling for invalid keys.

#### Acceptance Criteria

1. WHEN a valid PIN and valid base64-encoded 16-byte encryption key are provided, THE Element1_Encryptor `EncryptElement1` function SHALL return a non-empty base64-encoded ciphertext and nil error
2. WHEN the encryption key decodes to a length other than 16 bytes, THE Element1_Encryptor `EncryptElement1` function SHALL return an error containing "must decode to 16 bytes"
3. WHEN the encryption key is not valid base64, THE Element1_Encryptor `EncryptElement1` function SHALL return an error containing "base64 decode"
4. FOR ALL valid PIN and key pairs, encrypting the same PIN with the same key SHALL produce the same ciphertext (deterministic encryption due to fixed IV)

### Requirement 5: MSISDN Normalization and Helper Function Unit Tests

**User Story:** As a developer, I want unit tests for MSISDN normalization, sequence derivation, and transaction ID building, so that I can verify correct format transformations and edge case handling.

#### Acceptance Criteria

1. WHEN an MSISDN starting with "0" is provided, THE MSISDN_Normalizer SHALL replace the leading "0" with "62" and return a 13-character string
2. WHEN an MSISDN starting with "+62" is provided, THE MSISDN_Normalizer SHALL strip the "+" prefix and return a 13-character string starting with "62"
3. WHEN an MSISDN starting with "62" is provided, THE MSISDN_Normalizer SHALL return the value unchanged if it is 13 characters
4. WHEN an empty MSISDN is provided, THE MSISDN_Normalizer SHALL return an error containing "msisdn is required"
5. WHEN the normalized MSISDN is not 13 characters, THE MSISDN_Normalizer SHALL return an error containing "must be 13 characters"
6. WHEN a numeric string msgID is provided, THE `deriveSequence` function SHALL return the integer value modulo 10000
7. WHEN an empty msgID is provided, THE `deriveSequence` function SHALL return a value derived from the atomic counter modulo 10000
8. WHEN a non-numeric msgID is provided, THE `deriveSequence` function SHALL return a SHA1-derived value modulo 10000
9. THE `buildTelkomselTransactionID` function SHALL return a string composed of a 6-character org code, a 15-character timestamp, and a 4-digit zero-padded sequence

### Requirement 6: Consume Wrapper Function Unit Tests

**User Story:** As a developer, I want unit tests for the consume wrapper functions, so that I can verify environment variable validation, parameter normalization, and correct delegation to the Telkomsel client.

#### Acceptance Criteria

1. WHEN all required environment variables are set and valid parameters are provided, THE Consume_Wrapper `InitiateRegularRechargeOnConsume` function SHALL call the Telkomsel API and return a response
2. WHEN any required environment variable is missing, THE Consume_Wrapper `InitiateRegularRechargeOnConsume` function SHALL return an error identifying the missing variable
3. WHEN the `mid` parameter is empty, THE Consume_Wrapper `InitiateRegularRechargeOnConsume` function SHALL return an error containing "mid is required"
4. WHEN the `amount` parameter is zero or negative, THE Consume_Wrapper `InitiateRegularRechargeOnConsume` function SHALL return an error containing "amount must be greater than zero"
5. WHEN all required environment variables are set and valid parameters are provided, THE Consume_Wrapper `BrowseOfferOnConsume` function SHALL call the Telkomsel API and return a response
6. WHEN the `product_id` parameter is empty, THE Consume_Wrapper `BrowseOfferOnConsume` function SHALL return an error containing "product_id is required"
7. WHEN all required environment variables are set and valid parameters are provided, THE Consume_Wrapper `OrderDealerOnConsume` function SHALL call the Telkomsel API and return a response
8. WHEN all required environment variables are set and valid parameters are provided, THE Consume_Wrapper `CheckOrderStatusOnConsume` function SHALL call the Telkomsel API and return a response
9. WHEN the `original_transaction_id` parameter is empty, THE Consume_Wrapper `CheckOrderStatusOnConsume` function SHALL return an error containing "original_transaction_id is required"

### Requirement 7: Callback Handler Unit Tests

**User Story:** As a developer, I want unit tests for the callback handler, so that I can verify query parameter validation, transaction lookup, response logging, downstream publishing, and correct HTTP responses.

#### Acceptance Criteria

1. WHEN a valid callback request with all required query parameters is received, THE Callback_Handler SHALL return HTTP 200 with `{"status":"OK","message":"Callback received"}`
2. WHEN the `transaction_id` query parameter is missing, THE Callback_Handler SHALL return HTTP 400 with an error message containing "transaction_id is required"
3. WHEN the `organization_code` query parameter length is outside the 6-13 character range, THE Callback_Handler SHALL return HTTP 400 with an error message describing the length constraint
4. WHEN the `service_id` query parameter is not 13 characters, THE Callback_Handler SHALL return HTTP 400 with an error message describing the length constraint
5. WHEN the `status` query parameter is not "SUCCESS" or "FAILED", THE Callback_Handler SHALL return HTTP 400 with an error message listing valid values
6. WHEN the `message` query parameter is missing, THE Callback_Handler SHALL return HTTP 400 with an error message containing "message is required"
7. WHEN a valid callback is received and a Transaction_Logger is configured, THE Callback_Handler SHALL call `GetTransactionByOurTrxID` and `InsertCallbackResponse` on the Transaction_Logger
8. WHEN a valid callback is received and an MQ_Publisher is configured with a non-empty `mq_transaction` URL, THE Callback_Handler SHALL publish a `ProviderPublishMessage` to the downstream queue
9. WHEN the Transaction_Logger lookup fails, THE Callback_Handler SHALL use the `transaction_id` as fallback `msg_id` and still return HTTP 200

### Requirement 8: HTTP Server Unit Tests

**User Story:** As a developer, I want unit tests for the HTTP server, so that I can verify route registration and health check endpoint behavior.

#### Acceptance Criteria

1. WHEN a GET request is sent to `/health`, THE HTTP_Server SHALL return HTTP 200
2. WHEN a GET request is sent to `/callback/ext` with valid query parameters, THE HTTP_Server SHALL route the request to the Callback_Handler

### Requirement 9: Consumer Service Payload Parsing Unit Tests

**User Story:** As a developer, I want unit tests for the RabbitMQ consumer payload parsing, so that I can verify flexible JSON deserialization handles all field name variants and command parsing formats.

#### Acceptance Criteria

1. WHEN a JSON payload with snake_case field names is provided, THE Consume_Payload `UnmarshalJSON` method SHALL correctly parse all fields
2. WHEN a JSON payload with camelCase field names is provided, THE Consume_Payload `UnmarshalJSON` method SHALL correctly parse all fields
3. WHEN the `command` field contains a pulsa format "{nominal}*{stockType}", THE Consume_Payload `parseCommand` method SHALL extract amount and stock_type
4. WHEN the `command` field contains a paket data format "{nominal}*{BID}*{stockType}", THE Consume_Payload `parseCommand` method SHALL extract amount, product_id, and stock_type
5. WHEN the `amount` field is a JSON string instead of a number, THE Consume_Payload `UnmarshalJSON` method SHALL parse the string to an integer
6. WHEN the `msisdn` field is empty but `clientNumber` is present, THE Consume_Payload `UnmarshalJSON` method SHALL use `clientNumber` as the MSISDN fallback
7. WHEN the `typeVoucher` field is "pulsa", THE Consume_Payload `UnmarshalJSON` method SHALL derive `productType` as "pulsa"
8. WHEN the `typeVoucher` field is "paket data", THE Consume_Payload `UnmarshalJSON` method SHALL derive `productType` as "paket data"

### Requirement 10: MQ Publisher Message Construction Unit Tests

**User Story:** As a developer, I want unit tests for the MQ publisher message types, so that I can verify correct JSON serialization of downstream publish messages.

#### Acceptance Criteria

1. WHEN `NewProviderPublishMessage` is called with valid `ProviderPublishData`, THE Provider_Publish_Message SHALL have `source` set to "PROVIDER" and `data` matching the input
2. WHEN a `ProviderPublishMessage` is serialized to JSON, THE output SHALL contain `"source":"PROVIDER"` and all data fields with correct JSON keys

### Requirement 11: PostgreSQL API Log Repository Unit Tests

**User Story:** As a developer, I want unit tests for the API log repository helper functions, so that I can verify null handling and JSON conversion logic without requiring a database connection.

#### Acceptance Criteria

1. WHEN an empty string is passed to `nullIfEmpty`, THE function SHALL return nil
2. WHEN a non-empty string is passed to `nullIfEmpty`, THE function SHALL return the string value
3. WHEN zero is passed to `nullIfZero`, THE function SHALL return nil
4. WHEN a non-zero integer is passed to `nullIfZero`, THE function SHALL return the integer value
5. WHEN valid JSON bytes are passed to `toRawJSONB`, THE function SHALL return the bytes unchanged
6. WHEN invalid JSON bytes are passed to `toRawJSONB`, THE function SHALL return the bytes wrapped as a JSON string
7. WHEN an empty byte slice is passed to `toRawJSONB`, THE function SHALL return nil

### Requirement 12: PostgreSQL API Logger Adapter Unit Tests

**User Story:** As a developer, I want unit tests for the API logger adapter, so that I can verify it correctly maps `telkomsel.APICallLog` fields to `contractsvc.APILogEntry` and delegates to the repository.

#### Acceptance Criteria

1. WHEN `Log` is called with a valid `APICallLog`, THE API_Logger_Adapter SHALL call `Insert` on the underlying API_Log_Repository with a correctly mapped `APILogEntry`
2. WHEN the underlying `Insert` call returns an error, THE API_Logger_Adapter SHALL log the error using the Logger and not propagate the error

### Requirement 13: Telkomsel Client Utility Function Unit Tests

**User Story:** As a developer, I want unit tests for the client utility functions (signature generation, header sanitization, JSON redaction, error classification), so that I can verify security-sensitive logic and retry decisions.

#### Acceptance Criteria

1. WHEN valid `API_KEY`, `SECRET_KEY` environment variables and a timestamp are provided, THE `generateSignature` function SHALL return a 32-character hex MD5 hash
2. WHEN `API_KEY` or `SECRET_KEY` environment variable is missing, THE `generateSignature` function SHALL return an error
3. WHEN HTTP headers contain sensitive keys (api-key, x-signature, authorization), THE `sanitizeHeadersForLog` function SHALL mask the values
4. WHEN a JSON body contains `third_party_password` or `element1` fields, THE `sanitizeJSONForLog` function SHALL replace the values with "***"
5. WHEN a `BusinessError` is passed to `isRetryableError`, THE function SHALL return false
6. WHEN a `TechnicalError` with status code 500 is passed to `isRetryableError`, THE function SHALL return true
7. WHEN a `TechnicalError` wrapping a `net.Error` is passed to `isRetryableError`, THE function SHALL return true
8. WHEN `classifyError` is called with a `BusinessError`, THE function SHALL return error_type "business"
9. WHEN `classifyError` is called with a non-BusinessError, THE function SHALL return error_type "technical"
10. WHEN `classifyError` is called with nil, THE function SHALL return empty strings for both error_type and error_message

### Requirement 14: Coverage Target Enforcement

**User Story:** As a developer, I want the overall test suite to achieve at least 90% code coverage, so that I can have confidence in the correctness and maintainability of the codebase.

#### Acceptance Criteria

1. WHEN `go test ./... -coverprofile=coverage.out` is executed, THE Test_Suite SHALL produce a coverage report
2. THE Test_Suite SHALL achieve at least 90% statement coverage for the `internal/config` package
3. THE Test_Suite SHALL achieve at least 90% statement coverage for the `pkg/telkomsel` package
4. THE Test_Suite SHALL achieve at least 90% statement coverage for the `internal/handler` package
5. THE Test_Suite SHALL achieve at least 90% statement coverage for the `internal/infrastructure/mqpublisher` package
6. THE Test_Suite SHALL achieve at least 90% statement coverage for the `internal/infrastructure/postgres` package (for testable helper functions)
7. THE Test_Suite SHALL achieve at least 90% statement coverage for the `internal/infrastructure/rabbitmq` package (for payload parsing and helper functions)
8. THE Test_Suite SHALL achieve at least 90% statement coverage for the `internal/http` package (for route registration and health check)
