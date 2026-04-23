# Task Perbaikan Gateway UniPin (agar mirip pola Gateway Telkomsel)

Tanggal: 2026-04-17

Dokumen ini berisi daftar task perbaikan untuk menyamakan pola transaksi UniPin dengan Telkomsel: ada lifecycle transaksi yang jelas, retry/pending yang deterministik, dan payload downstream yang konsisten.

## High Priority

1) [DONE] Standarisasi status publish downstream (standar: F/C/S) — selesai 2026-04-17
- Problem: UniPin publish status_to_be = "SUCCESS/FAILED", Telkomsel publish status_to_be = F/C/S.
- Dampak: downstream consumer bisa salah interpretasi status antar provider.
- Target:
  - Tetapkan satu standar lintas gateway (pilih salah satu):
    - Opsi A: F/C/S (Finish/Cancel/Still processing)
    - Opsi B: SUCCESS/FAILED/PROCESSING
  - Implement mapping UniPin di proses publish (bukan di producer).
- Lokasi utama:
  - UniPin publish mapping: [internal/infrastructure/rabbitmq/consumer_service.go](../internal/infrastructure/rabbitmq/consumer_service.go#L733-L770)
  - UniPin transaction pending case: [internal/infrastructure/rabbitmq/consumer_service.go](../internal/infrastructure/rabbitmq/consumer_service.go#L705)
- Acceptance:
  - Downstream menerima status_to_be yang konsisten antar gateway.
  - Test unit publish payload di-update sesuai standar.

2) [DONE] Tambahkan mekanisme retry untuk status pending (publish S + retry async, max retry -> C) — selesai 2026-04-17
- Problem: pada UniPin, hasil OrderInquiry status selain 1/0 dianggap pending dan tidak publish apa pun (transaksi “menghilang”).
- Target:
  - Tambahkan retry loop async (mirip retryCheckStatus di Telkomsel) untuk OrderInquiry pending.
  - Retry harus configurable via env (mis. RETRY_MAX_ATTEMPTS, RETRY_WAIT_SECONDS).
  - Setelah max retry: publish FAILED (atau publish status pending final, tergantung standar).
- Lokasi utama:
  - Pending skip publish: [internal/infrastructure/rabbitmq/consumer_service.go](../internal/infrastructure/rabbitmq/consumer_service.go#L705)
  - Entry retry env: tambahkan di [internal/config/config.go](../internal/config/config.go)
- Acceptance:
  - Pending tidak berhenti silent; ada final outcome atau minimal status processing/pending yang ter-publish.

3) [DONE] Benahi correlation fields payload downstream (msgid vs serial/reference) — selesai 2026-04-17
- Problem: UniPin mengisi ConversationID dengan serialNumber (referenceNo), bukan msgid; msg_id juga dipaksa int (kalau msgid non-numeric jadi 0).
- Target:
  - Jadikan msgid sebagai correlation utama.
  - Serial/reference UniPin taruh di SerialNumber atau field dedicated.
  - Jika msgid non-numeric, tetap preserve di field string (mis. ConversationID / OriginalConversationID) agar tidak hilang.
- Lokasi utama:
  - Mapping publish: [internal/infrastructure/rabbitmq/consumer_service.go](../internal/infrastructure/rabbitmq/consumer_service.go#L733-L770)
- Acceptance:
  - Downstream bisa selalu trace transaksi dengan msgid, walaupun msgid bukan angka.

4) [DONE] Perkuat parsing payload agar kompatibel variasi producer — selesai 2026-04-18
- Problem: consumePayload hanya membaca key "command"; variasi seperti "Command" tidak kebaca → command kosong → transaksi gagal.
- Target:
  - Tambahkan alias key untuk field penting (command, msgid, queue_name, mq_transaction, dll) seperti pendekatan Telkomsel.
- Lokasi utama:
  - Unmarshal command: [internal/infrastructure/rabbitmq/consumer_service.go](../internal/infrastructure/rabbitmq/consumer_service.go#L78)
- Acceptance:
  - Payload dengan variasi casing/alias masih berhasil diparse.

5) [DONE] Samakan strategi MQ publish agar aman terhadap queue property mismatch — selesai 2026-04-18
- Problem: UniPin publisher melakukan QueueDeclare (durable lalu fallback non-durable). Ini rawan PRECONDITION_FAILED dan menambah coupling dengan infra.
- Target:
  - Ikuti pola Telkomsel: jangan declare queue saat publish; gunakan mandatory publish + detect unroutable (NotifyReturn) bila perlu.
- Lokasi utama:
  - UniPin publisher: [internal/infrastructure/mqpublisher/publisher.go](../internal/infrastructure/mqpublisher/publisher.go#L50-L66)
- Acceptance:
  - Publish tidak gagal karena mismatch deklarasi queue.
  - Jika queue tidak ada, error bisa terdeteksi jelas.

## Low Priority

1) [DONE] Rapikan lifecycle service dengan errgroup (fail-fast & graceful shutdown) — selesai 2026-04-18
- Problem: UniPin menjalankan HTTP server & consumer via goroutine terpisah tanpa koordinasi error yang rapi.
- Target:
  - Gunakan errgroup untuk menjalankan HTTP, consumer, dan scheduler; kalau satu fatal, cancel context.
- Lokasi utama:
  - Startup goroutine: [cmd/app/main.go](../cmd/app/main.go#L140-L170)

2) [DONE] Tambahkan middleware recover di Fiber — selesai 2026-04-18
- Problem: tanpa recover, panic handler bisa bikin request drop/terminate chain.
- Target:
  - Tambahkan fiber recover middleware.
- Lokasi utama:
  - Fiber init: [cmd/app/main.go](../cmd/app/main.go#L120-L170)

3) [DONE] Konsistensi naming TypeVoucher/ProductType — selesai 2026-04-18
- Problem: UniPin melakukan mapping txType dari TypeVoucher lalu fallback dari ProductType (backward compat).
- Target:
  - Definisikan satu field sumber kebenaran (mis. typeVoucher), dan dokumentasikan formatnya.
- Lokasi utama:
  - Switch txType: [internal/infrastructure/rabbitmq/consumer_service.go](../internal/infrastructure/rabbitmq/consumer_service.go#L342-L370)

4) [DONE] Dokumentasi flow resmi UniPin (setara flow Telkomsel) — selesai 2026-04-18
- Target:
  - Buat docs flowchart Mermaid UniPin untuk on-boarding dan audit.
- Lokasi:
  - docs/flow_gateway_unipin.md
