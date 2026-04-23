# Implementation Plan: Paket Data Post-Publish Check Status

## Overview

Implementasi goroutine check status baru untuk flow "paket data" yang mengecek status transaksi di database sebelum memanggil Telkomsel Check Order Status API. Perubahan dilakukan secara incremental: mulai dari interface & DB layer, lalu goroutine baru, dan terakhir integrasi ke paket data flow.

## Tasks

- [x] 1. Add `GetTransactionStatusByMsgID` to TransactionLogger interface and implement in Postgres
  - [x] 1.1 Add `GetTransactionStatusByMsgID` method signature to `TransactionLogger` interface
    - Add `GetTransactionStatusByMsgID(ctx context.Context, msgID string) (string, error)` to the interface in `internal/domain/contract/service/transaction_logger.go`
    - _Requirements: 1.1_
  - [x] 1.2 Implement `GetTransactionStatusByMsgID` in `PostgresTransactionLogger`
    - Add SQL constant `getTransactionStatusByMsgIDSQL` with query `SELECT status FROM transaction.telkomsel_transaction WHERE msg_id = $1 LIMIT 1`
    - Implement the method using `QueryRowContext` and handle `sql.ErrNoRows` with a descriptive wrapped error
    - File: `internal/infrastructure/postgres/transaction_logger.go`
    - _Requirements: 1.1, 1.2, 1.3, 1.4_
  - [x] 1.3 Add "PROCESSING" status support to `UpdateTransactionStatus`
    - Add a new `updateStatusProcessingSQL` constant: `UPDATE transaction.telkomsel_transaction SET status = 'PROCESSING', updated_at = NOW() WHERE msg_id = $1`
    - Add `case "PROCESSING"` to the switch statement in `UpdateTransactionStatus`
    - Ensure existing "SUCCESS" and "FAILED" cases remain unchanged
    - File: `internal/infrastructure/postgres/transaction_logger.go`
    - _Requirements: 8.1, 8.2, 8.3_
  - [ ]* 1.4 Write property test: Invalid status rejected by UpdateTransactionStatus
    - **Property 3: Invalid status rejected by UpdateTransactionStatus**
    - Use `pgregory.net/rapid` to generate random strings that are NOT "PROCESSING", "SUCCESS", or "FAILED"
    - Call `UpdateTransactionStatus` with the generated string and verify non-nil error is returned
    - Minimum 100 iterations
    - File: `internal/infrastructure/postgres/transaction_logger_property_test.go`
    - **Validates: Requirements 8.3**

- [x] 2. Checkpoint — Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 3. Implement `retryCheckStatusPaketData` and `retryCheckStatusPaketDataSync` methods
  - [x] 3.1 Create `retryCheckStatusPaketData` async launcher method on `ConsumerServiceImpl`
    - Add method that logs info (queue, msisdn, mid, msgid) and launches `go s.retryCheckStatusPaketDataSync(context.Background(), ...)` 
    - Parameters: `ctx context.Context, payload consumePayload, msgID string, queueName string, requestedAt time.Time, originalTransactionID string, serialNumber string`
    - File: `internal/infrastructure/rabbitmq/consumer_service.go`
    - _Requirements: 2.3, 2.4, 7.1_
  - [x] 3.2 Create `retryCheckStatusPaketDataSync` synchronous retry loop method on `ConsumerServiceImpl`
    - Parameters: `ctx context.Context, payload consumePayload, msgID string, queueName string, requestedAt time.Time, originalTransactionID string, serialNumber string`
    - Handle nil `retryConfig`: treat as FAILED immediately, publish `StatusToBeCancel`, return
    - Loop up to `MaxAttempts` iterations:
      1. `time.Sleep(WaitDuration)`
      2. Call `GetTransactionStatusByMsgID(msgID)` — if SUCCESS/FAILED, log & return; if error, log & fallback to API call
      3. Call `CheckOrderStatusOnConsume(ctx, msisdn, mid, queueName, msgID, originalTransactionID, serialNumber)`
      4. Resolve rcPPS: 0 → update SUCCESS + publish StatusToBeFinish + return; 1 → processRetryFailed + return; 9 → continue
    - After max attempts: update FAILED, publish StatusToBeCancel with "pending: max retry reached"
    - Log at each stage per Requirements 7.1–7.5
    - File: `internal/infrastructure/rabbitmq/consumer_service.go`
    - _Requirements: 2.5, 3.1, 3.2, 4.1, 4.2, 4.3, 4.4, 5.1, 5.2, 5.3, 5.4, 5.5, 6.1, 6.2, 6.3, 7.1, 7.2, 7.3, 7.4, 7.5_
  - [ ]* 3.3 Write property test: Resolved status stops goroutine without API call
    - **Property 1: Resolved status stops goroutine without API call or downstream publish**
    - Use `pgregory.net/rapid` to generate random resolved status (SUCCESS/FAILED)
    - Mock `GetTransactionStatusByMsgID` to return the generated status
    - Verify `CheckOrderStatusOnConsume` is NOT called and no downstream MQ publish occurs
    - Minimum 100 iterations
    - File: `internal/infrastructure/rabbitmq/consumer_service_property_test.go`
    - **Validates: Requirements 4.2**
  - [ ]* 3.4 Write property test: Retry loop bounded by MaxAttempts
    - **Property 2: Retry loop bounded by MaxAttempts**
    - Use `pgregory.net/rapid` to generate random `MaxAttempts` (1–20)
    - Mock DB to always return PROCESSING, mock API to always return rcPPS 9
    - Verify exactly `MaxAttempts` API calls are made
    - Verify FAILED status update and `StatusToBeCancel` publish after last attempt
    - Minimum 100 iterations
    - File: `internal/infrastructure/rabbitmq/consumer_service_property_test.go`
    - **Validates: Requirements 6.1, 6.2, 6.3**
  - [ ]* 3.5 Write unit tests for `retryCheckStatusPaketDataSync`
    - Test rcPPS == 0 path: DB returns PROCESSING → API returns rcPPS 0 → verify SUCCESS update & StatusToBeFinish publish
    - Test rcPPS == 1 path: DB returns PROCESSING → API returns rcPPS 1 → verify FAILED update & StatusToBeCancel publish
    - Test rcPPS == 9 then resolved by callback: iteration 1 DB PROCESSING + API rcPPS 9, iteration 2 DB SUCCESS → verify stop without API call
    - Test nil retryConfig: verify immediate FAILED treatment and StatusToBeCancel publish
    - Test DB error fallback: GetTransactionStatusByMsgID returns error → verify API call still happens
    - Test API error without response: CheckOrderStatusOnConsume returns error with nil response → verify continues to next iteration
    - File: `internal/infrastructure/rabbitmq/consumer_service_test.go`
    - _Requirements: 3.2, 4.2, 4.3, 4.4, 5.2, 5.3, 5.4, 5.5, 6.1, 6.2, 6.3_

- [x] 4. Checkpoint — Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Integrate new goroutine into paket data flow
  - [x] 5.1 Wire `retryCheckStatusPaketData` into rcPPS == 0 case in paket data flow
    - In `consumeSession`, under `case "paket data"` → `case 0:` (after `publishToDownstream` with `StatusToBeProcess`), add call to `s.retryCheckStatusPaketData(ctx, payload, msgID, queueName, orderRequestedAt, orderResp.Transaction.TransactionID, "")`
    - Replace the existing TODO comment block with the actual goroutine call
    - File: `internal/infrastructure/rabbitmq/consumer_service.go`
    - _Requirements: 2.1, 2.5_
  - [x] 5.2 Wire `retryCheckStatusPaketData` into rcPPS == 9 case in paket data flow
    - In `consumeSession`, under `case "paket data"` → `case 9:`, replace `s.retryCheckStatus(ctx, payload, msgID, queueName, orderRequestedAt, nil)` with `s.retryCheckStatusPaketData(ctx, payload, msgID, queueName, orderRequestedAt, orderResp.Transaction.TransactionID, "")`
    - Remove the old TODO comment about "check status dengan tambahan logic get latest status di transaction"
    - File: `internal/infrastructure/rabbitmq/consumer_service.go`
    - _Requirements: 2.2, 2.5_
  - [ ]* 5.3 Write unit tests for paket data flow integration
    - Test rcPPS == 0: verify `retryCheckStatusPaketData` is called (not `retryCheckStatus`)
    - Test rcPPS == 9: verify `retryCheckStatusPaketData` is called (not `retryCheckStatus`)
    - File: `internal/infrastructure/rabbitmq/consumer_service_test.go`
    - _Requirements: 2.1, 2.2_

- [x] 6. Final checkpoint — Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests use `pgregory.net/rapid` with minimum 100 iterations per property
- The existing `retryCheckStatus`/`retryCheckStatusSync` methods for pulsa flow remain unchanged
- `originalTransactionID` uses `orderResp.Transaction.TransactionID` from OrderDealer response
- `serialNumber` uses empty string `""` as it's not available from OrderDealer response for paket data
