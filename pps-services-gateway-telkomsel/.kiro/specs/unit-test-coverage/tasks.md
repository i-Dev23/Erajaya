# Implementation Plan: Unit Test Coverage

## Overview

Implement comprehensive unit tests across all packages of pps-services-gateway-telkomsel to achieve ≥90% statement coverage. Tests are organized by package, starting with foundational helpers and mocks, then building up to integration-level tests. Each task creates or modifies test files, uses table-driven patterns, httptest mock servers, fiber test utilities, and hand-written mocks. Property-based tests use `pgregory.net/rapid`.

## Tasks

- [x] 1. Add `pgregory.net/rapid` dependency
  - Run `go get pgregory.net/rapid` to add the PBT library to `go.mod`
  - _Requirements: 14.1_

- [x] 2. Implement `internal/config` package tests
  - [x] 2.1 Create `internal/config/config_test.go` with table-driven tests for `Load`, `LoadCallbackServer`, and `LoadTelkomsel`
    - Test `Load` with all valid env vars returns correct `Config` struct
    - Test `Load` with missing `RABBITMQ_URL` returns error containing "RABBITMQ_URL is required"
    - Test `Load` with empty `QUEUE_NAME_PROVIDER` but set `QUEUE_NAME` uses fallback
    - Test `Load` with empty `CONSUMER_TAG` defaults to "pps-services-gateway-telkomsel-consumer"
    - Test `LoadCallbackServer` with empty `CALLBACK_PORT` defaults to 8080
    - Test `LoadCallbackServer` with non-numeric `CALLBACK_PORT` returns error
    - Test `LoadTelkomsel` with all valid env vars returns correct `TelkomselConfig`
    - Test `LoadTelkomsel` with each missing required env var returns error identifying the variable
    - Test `LoadTelkomsel` with valid positive `TIMEOUT` parses correctly
    - Test `LoadTelkomsel` with zero or negative `TIMEOUT` returns error
    - Use `t.Setenv` for env var isolation per subtest (no `t.Parallel`)
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9, 1.10_

  - [ ]* 2.2 Write property test for config env var mapping (Property 1)
    - **Property 1: Config loader env var mapping**
    - Use `rapid.Check` to generate random valid env var values and verify `Load`/`LoadTelkomsel` return matching struct fields
    - **Validates: Requirements 1.1, 1.7**

  - [ ]* 2.3 Write property test for LoadTelkomsel missing env var identification (Property 2)
    - **Property 2: LoadTelkomsel missing env var identification**
    - For each required env var, unset it while others are set, verify error message contains the variable name
    - **Validates: Requirements 1.8**

  - [ ]* 2.4 Write property test for config timeout parsing (Property 3)
    - **Property 3: Config timeout parsing**
    - Generate random positive integers, set as `TIMEOUT`, verify `TelkomselConfig.Timeout` matches
    - **Validates: Requirements 1.9**

- [x] 3. Checkpoint - Ensure config tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. Implement `pkg/telkomsel` helper function tests
  - [x] 4.1 Create `pkg/telkomsel/helpers_test.go` with table-driven tests for unexported helper functions
    - Test `normalizeMSISDN` with "0"-prefix, "+62"-prefix, "62"-prefix, empty, and invalid-length inputs
    - Test `deriveSequence` with numeric, empty, and non-numeric msgID inputs
    - Test `buildTelkomselTransactionID` with various org codes, timestamps, and sequence values; verify 25-char output format
    - Test `organizationCodeAndPINFromMIDEnv` with valid, missing, and malformed env values
    - Test `generateSignature` with valid env vars returns 32-char hex; test with missing env vars returns error
    - Test `sanitizeHeadersForLog` masks sensitive keys (api-key, x-signature, authorization) and passes others through
    - Test `sanitizeJSONForLog` redacts `third_party_password` and `element1` fields with "***"
    - Test `classifyError` with `BusinessError`, non-`BusinessError`, and nil
    - Test `isRetryableError` with `BusinessError` (false), `TechnicalError` with 500 (true), `TechnicalError` wrapping `net.Error` (true)
    - Test `maskForLog` and `truncateForLog` edge cases
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 5.8, 5.9, 13.1, 13.2, 13.3, 13.4, 13.5, 13.6, 13.7, 13.8, 13.9, 13.10_

  - [ ]* 4.2 Write property test for MSISDN normalization (Property 9)
    - **Property 9: MSISDN normalization format invariant**
    - Generate valid MSISDNs with "0", "+62", "62" prefixes that normalize to 13 digits; verify output is 13 chars starting with "62"
    - **Validates: Requirements 5.1, 5.2, 5.3**

  - [ ]* 4.3 Write property test for deriveSequence range (Property 10)
    - **Property 10: deriveSequence range invariant**
    - Generate arbitrary strings; verify `deriveSequence` returns value in [0, 9999]
    - **Validates: Requirements 5.6, 5.7, 5.8**

  - [ ]* 4.4 Write property test for buildTelkomselTransactionID structure (Property 11)
    - **Property 11: buildTelkomselTransactionID structure invariant**
    - Generate random org codes, timestamps, sequences; verify 25-char output with correct segments
    - **Validates: Requirements 5.9**

  - [ ]* 4.5 Write property test for generateSignature hex output (Property 22)
    - **Property 22: generateSignature produces 32-char hex**
    - Generate random API_KEY, SECRET_KEY, timestamp; verify 32-char hex output
    - **Validates: Requirements 13.1**

  - [ ]* 4.6 Write property test for sanitizeHeadersForLog masking (Property 23)
    - **Property 23: sanitizeHeadersForLog masks sensitive keys**
    - Generate header maps with sensitive keys; verify values are masked
    - **Validates: Requirements 13.3**

  - [ ]* 4.7 Write property test for sanitizeJSONForLog redaction (Property 24)
    - **Property 24: sanitizeJSONForLog redacts sensitive fields**
    - Generate JSON objects with `third_party_password`/`element1`; verify values replaced with "***"
    - **Validates: Requirements 13.4**

  - [ ]* 4.8 Write property test for error classification (Property 25)
    - **Property 25: Error classification correctness**
    - Verify `classifyError` returns "business" for `BusinessError`, "technical" for other errors, empty for nil
    - **Validates: Requirements 13.8, 13.9, 13.10**

- [x] 5. Implement `pkg/telkomsel` Element1 encryption tests
  - [x] 5.1 Create `pkg/telkomsel/element1_test.go` with table-driven tests
    - Test `EncryptElement1` with valid PIN and valid 16-byte base64 key returns non-empty base64 ciphertext
    - Test `EncryptElement1` with key that decodes to ≠16 bytes returns error containing "must decode to 16 bytes"
    - Test `EncryptElement1` with invalid base64 key returns error containing "base64 decode"
    - Test determinism: same PIN + same key produces identical ciphertext
    - Test `pkcs5Pad` with various input lengths and block sizes
    - _Requirements: 4.1, 4.2, 4.3, 4.4_

  - [ ]* 5.2 Write property test for EncryptElement1 valid base64 output (Property 6)
    - **Property 6: EncryptElement1 produces valid base64 output**
    - Generate random non-empty PINs and valid 16-byte base64 keys; verify output is valid base64
    - **Validates: Requirements 4.1**

  - [ ]* 5.3 Write property test for EncryptElement1 determinism (Property 7)
    - **Property 7: EncryptElement1 determinism**
    - Generate random PIN/key pairs; call twice; verify identical output
    - **Validates: Requirements 4.4**

  - [ ]* 5.4 Write property test for invalid encryption key rejection (Property 8)
    - **Property 8: Invalid encryption key rejection**
    - Generate base64 strings decoding to ≠16 bytes; verify error contains "must decode to 16 bytes"
    - **Validates: Requirements 4.2**

- [x] 6. Implement `pkg/telkomsel` client construction and validation tests
  - [x] 6.1 Create `pkg/telkomsel/client_test.go` with table-driven tests for `NewClient` and all validators
    - Test `NewClient` with valid params returns non-nil Client
    - Test `NewClient` with each empty required param returns error identifying the param
    - Test `NewClient` with zero/negative timeout returns error
    - Test `validateInitiateRegularRechargeRequest` with valid request returns nil
    - Test `validateInitiateRegularRechargeRequest` with transaction_id > 25 chars, empty fields, invalid service_id, invalid stock_type
    - Test `validateOrderDealerRequest` with valid and invalid inputs
    - Test `validateBrowseOfferRequest` with valid and invalid inputs
    - Test `validateCheckOrderStatusRequest` with valid and invalid inputs
    - Test `buildBrowseOfferURL` and `buildCheckOrderStatusURL` produce correct query strings
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8, 2.9, 2.10, 2.11_

  - [ ]* 6.2 Write property test for valid request validation acceptance (Property 4)
    - **Property 4: Valid request validation acceptance**
    - Generate well-formed requests (all fields populated, service_id=13 chars starting "62", transaction_id ≤25 chars, stock_type ∈ {"FIXED","BULK"}); verify validators return nil
    - **Validates: Requirements 2.4, 2.8, 2.9, 2.10**

  - [ ]* 6.3 Write property test for invalid request field identification (Property 5)
    - **Property 5: Invalid request field identification**
    - For each request type, set one required field to empty; verify error message contains the field name
    - **Validates: Requirements 2.2, 2.5, 2.6, 2.7, 2.11**

- [x] 7. Implement `pkg/telkomsel` client API method tests with httptest
  - [x] 7.1 Add API method tests to `pkg/telkomsel/client_test.go` using `httptest.NewServer`
    - Test `InitiateRegularRecharge` with HTTP 200 + status_code "00000" returns parsed response
    - Test `InitiateRegularRecharge` with HTTP 200 + non-"00000" status_code returns `BusinessError`
    - Test `InitiateRegularRecharge` with HTTP 500 returns `TechnicalError`
    - Test `BrowseOffer` with HTTP 200 returns parsed `BrowseOfferResponse` with product details
    - Test `BrowseOffer` with HTTP 400 + non-"00000" status_code returns `BusinessError` and parsed response
    - Test `OrderDealer` with HTTP 200 returns parsed `OrderDealerResponse`
    - Test `CheckOrderStatus` with HTTP 200 returns parsed `CheckOrderStatusResponse`
    - Test `CheckOrderStatus` with HTTP 400 + non-"00000" status_code returns `BusinessError`
    - Test retry behavior: HTTP 500 retries up to `maxRetries` times
    - Test no retry on `BusinessError`
    - Verify request headers (api-key, Channel-Id, Timestamp, External-Transaction-Id, x-signature) are set correctly
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8, 3.9, 3.10_

  - [ ]* 7.2 Write property test for non-success status code produces BusinessError (Property 12)
    - **Property 12: Non-success status code produces BusinessError**
    - Generate random non-"00000" status codes; mock server returns them; verify `BusinessError.Code` matches
    - **Validates: Requirements 3.2**

- [x] 8. Checkpoint - Ensure pkg/telkomsel tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 9. Implement `pkg/telkomsel` consume wrapper tests
  - [x] 9.1 Create `pkg/telkomsel/order_dealer_consume_test.go` with httptest + t.Setenv tests
    - Test `OrderDealerOnConsume` with all valid env vars and params returns response
    - Test with each missing required env var returns error identifying the variable
    - Test with empty `mid` returns error
    - Test with invalid `stock_type` returns error
    - Test with empty `product_id` returns error
    - Verify request body contains correct transaction_id, channel, organization_code, service_id, product_id, stock_type, element1
    - _Requirements: 6.7_

  - [x] 9.2 Create `pkg/telkomsel/check_order_status_consume_test.go` with httptest + t.Setenv tests
    - Test `CheckOrderStatusOnConsume` with all valid env vars and params returns response
    - Test with each missing required env var returns error
    - Test with empty `original_transaction_id` returns error
    - Test with empty `mid` returns error
    - _Requirements: 6.8, 6.9_

  - [ ]* 9.3 Write additional unit tests for existing consume wrappers
    - Extend `consume_pulsa_test.go` with missing env var error cases for `InitiateRegularRechargeOnConsume`
    - Test `InitiateRegularRechargeOnConsume` with empty `mid`, zero `amount`, invalid `stock_type`
    - Test `BrowseOfferOnConsume` with missing env vars and empty `product_id`
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6_

- [x] 10. Implement `internal/infrastructure/rabbitmq` payload parsing tests
  - [x] 10.1 Create `internal/infrastructure/rabbitmq/consumer_payload_test.go` with table-driven tests
    - Test `consumePayload.UnmarshalJSON` with snake_case field names
    - Test `consumePayload.UnmarshalJSON` with camelCase field names
    - Test `parseCommand` with pulsa format "{nominal}*{stockType}" extracts amount and stock_type
    - Test `parseCommand` with paket data format "{nominal}*{BID}*{stockType}" extracts amount, product_id, stock_type
    - Test `amount` as JSON string parses to integer
    - Test `msisdn` empty with `clientNumber` present uses fallback
    - Test `typeVoucher` "pulsa" derives `productType` "pulsa"
    - Test `typeVoucher` "paket data" derives `productType` "paket data"
    - Test `getAny` with nil map, missing keys, present keys
    - Test `parseString` with nil, string, json.Number, float64 inputs
    - Test `parseInt` with nil, json.Number, float64, string inputs
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6, 9.7, 9.8_

  - [ ]* 10.2 Write property test for flexible field name parsing (Property 13)
    - **Property 13: Consume payload flexible field name parsing**
    - Generate valid payload data; encode as snake_case and camelCase JSON; verify identical parsed values
    - **Validates: Requirements 9.1, 9.2**

  - [ ]* 10.3 Write property test for pulsa command parsing (Property 14)
    - **Property 14: Pulsa command parsing extraction**
    - Generate random positive nominal and stock type; format as "{nominal}*{stockType}"; verify extraction
    - **Validates: Requirements 9.3**

  - [ ]* 10.4 Write property test for paket data command parsing (Property 15)
    - **Property 15: Paket data command parsing extraction**
    - Generate random nominal, product ID, stock type; format as "{nominal}*{BID}*{stockType}"; verify extraction
    - **Validates: Requirements 9.4**

  - [ ]* 10.5 Write property test for amount string-to-int parsing (Property 16)
    - **Property 16: Amount string-to-int parsing**
    - Generate non-negative integers; encode as JSON string in `amount` field; verify parsed `Amount` matches
    - **Validates: Requirements 9.5**

  - [ ]* 10.6 Write property test for clientNumber MSISDN fallback (Property 17)
    - **Property 17: clientNumber MSISDN fallback**
    - Generate non-empty clientNumber with empty msisdn; verify `MSISDN` equals clientNumber
    - **Validates: Requirements 9.6**

- [x] 11. Implement `internal/infrastructure/mqpublisher` message tests
  - [x] 11.1 Create `internal/infrastructure/mqpublisher/message_test.go` with table-driven tests
    - Test `NewProviderPublishMessage` sets `Source` to "PROVIDER" and `Data` matches input
    - Test JSON serialization contains `"source":"PROVIDER"` and all data fields with correct keys
    - Test with various `ProviderPublishData` field combinations
    - _Requirements: 10.1, 10.2_

  - [ ]* 11.2 Write property test for ProviderPublishMessage serialization (Property 18)
    - **Property 18: ProviderPublishMessage serialization round-trip**
    - Generate random `ProviderPublishData`; create message; serialize to JSON; verify `"source":"PROVIDER"` and data fields match
    - **Validates: Requirements 10.1, 10.2**

- [x] 12. Checkpoint - Ensure infrastructure and consume wrapper tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 13. Implement `internal/infrastructure/postgres` helper and adapter tests
  - [x] 13.1 Create `internal/infrastructure/postgres/api_log_helpers_test.go` with table-driven tests
    - Test `nullIfEmpty` with empty string returns nil, non-empty returns string
    - Test `nullIfZero` with zero returns nil, non-zero returns integer
    - Test `toRawJSONB` with valid JSON returns bytes unchanged
    - Test `toRawJSONB` with invalid JSON returns JSON-wrapped string
    - Test `toRawJSONB` with empty slice returns nil
    - Test `toJSONB` with nil and non-nil inputs
    - _Requirements: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7_

  - [x] 13.2 Create `internal/infrastructure/postgres/mock_test.go` with mock implementations
    - Implement `mockAPILogRepository` with configurable `Insert` return error and call capture
    - Implement `mockLogger` with call capture for `Info`, `Warn`, `Error`
    - _Requirements: 12.1, 12.2_

  - [x] 13.3 Create `internal/infrastructure/postgres/api_logger_adapter_test.go` with mock-based tests
    - Test `APILoggerAdapter.Log` calls `Insert` on mock repo with correctly mapped `APILogEntry` fields
    - Test `APILoggerAdapter.Log` when `Insert` returns error logs the error via mock logger
    - Verify all field mappings from `telkomsel.APICallLog` to `contractsvc.APILogEntry`
    - _Requirements: 12.1, 12.2_

  - [ ]* 13.4 Write property test for null helper passthrough (Property 19)
    - **Property 19: Null helper passthrough**
    - Generate non-empty strings; verify `nullIfEmpty` returns unchanged. Generate non-zero ints; verify `nullIfZero` returns unchanged
    - **Validates: Requirements 11.2, 11.4**

  - [ ]* 13.5 Write property test for toRawJSONB valid JSON passthrough (Property 20)
    - **Property 20: toRawJSONB valid JSON passthrough**
    - Generate valid JSON byte slices; verify `toRawJSONB` returns bytes unchanged
    - **Validates: Requirements 11.5**

  - [ ]* 13.6 Write property test for toRawJSONB invalid JSON wrapping (Property 21)
    - **Property 21: toRawJSONB invalid JSON wrapping**
    - Generate non-empty non-JSON byte slices; verify `toRawJSONB` returns valid JSON
    - **Validates: Requirements 11.6**

- [x] 14. Implement `internal/handler` callback handler tests
  - [x] 14.1 Create `internal/handler/mock_test.go` with mock implementations
    - Implement `mockLogger` with call capture
    - Implement `mockTransactionLogger` with configurable returns for `GetTransactionByOurTrxID`, `InsertCallbackResponse`, and other methods
    - Implement `mockMQPublisher` with configurable `Publish` return and call capture
    - _Requirements: 7.7, 7.8_

  - [x] 14.2 Create `internal/handler/callback_handler_test.go` with fiber `app.Test()` tests
    - Test valid callback with all required params returns HTTP 200 with `{"status":"OK","message":"Callback received"}`
    - Test missing `transaction_id` returns HTTP 400
    - Test `organization_code` length outside 6-13 returns HTTP 400
    - Test `service_id` not 13 characters returns HTTP 400
    - Test `status` not "SUCCESS"/"FAILED" returns HTTP 400
    - Test missing `message` returns HTTP 400
    - Test with configured `TransactionLogger`: verify `GetTransactionByOurTrxID` and `InsertCallbackResponse` are called
    - Test with configured `MQPublisher` and non-empty `mq_transaction`: verify `Publish` is called with correct `ProviderPublishMessage`
    - Test when `TransactionLogger` lookup fails: uses `transaction_id` as fallback `msg_id`, still returns HTTP 200
    - Test with nil `MQPublisher` and nil `TransactionLogger` still returns HTTP 200
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7, 7.8, 7.9_

  - [ ]* 14.3 Write property test for callback handler invalid parameter rejection (Property 26)
    - **Property 26: Callback handler invalid parameter rejection**
    - Generate `organization_code` with length outside [6,13], `service_id` with length ≠13, `status` not in {"SUCCESS","FAILED"}; verify HTTP 400
    - **Validates: Requirements 7.3, 7.4, 7.5**

- [x] 15. Implement `internal/http` server tests
  - [x] 15.1 Create `internal/http/server_test.go` with fiber `app.Test()` tests
    - Test GET `/health` returns HTTP 200
    - Test GET `/callback/ext` with valid query params routes to callback handler (verify handler is invoked)
    - Use mock logger and mock callback handler dependencies
    - _Requirements: 8.1, 8.2_

- [x] 16. Checkpoint - Ensure all handler and server tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 17. Run full test suite with coverage and verify ≥90% per package
  - [x] 17.1 Run `go test ./... -coverprofile=coverage.out -count=1` and verify coverage
    - Check per-package coverage with `go tool cover -func=coverage.out`
    - Verify `internal/config` ≥90%
    - Verify `pkg/telkomsel` ≥90%
    - Verify `internal/handler` ≥90%
    - Verify `internal/infrastructure/mqpublisher` ≥90%
    - Verify `internal/infrastructure/postgres` ≥90% (for testable helpers)
    - Verify `internal/infrastructure/rabbitmq` ≥90% (for payload parsing)
    - Verify `internal/http` ≥90%
    - Add any missing test cases to close coverage gaps
    - _Requirements: 14.1, 14.2, 14.3, 14.4, 14.5, 14.6, 14.7, 14.8_

- [x] 18. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document (Properties 1–26)
- Unit tests validate specific examples and edge cases
- Tests use `t.Setenv` for env var isolation — do not use `t.Parallel()` in env-dependent tests
- Mock implementations are co-located in `mock_test.go` files within each package
- Existing test files (`consume_pulsa_test.go`, `browse_offer_test.go`, `browse_offer_consume_test.go`) are preserved and extended
