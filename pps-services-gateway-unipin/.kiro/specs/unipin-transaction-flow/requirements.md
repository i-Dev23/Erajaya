# Dokumen Requirements

## Pendahuluan

Fitur ini mengimplementasikan alur transaksi lengkap untuk dua product type di `pps-services-gateway-unipin`:

1. **unipin-game** (baru) — Alur in-game topup yang saat ini belum diimplementasikan (hanya menampilkan warning "not yet implemented"). Alur ini melibatkan parsing field `Command` untuk mendapatkan `gameCode` dan `denominationID`, parsing field `MSISDN` yang berisi JSON string untuk mendapatkan field validasi user, pemanggilan Validate User API, Create Order API, dan fallback ke Order Inquiry API saat timeout.

2. **unipin-voucher** (update) — Alur voucher yang sudah ada, namun perlu diubah sumber `denominationCode`-nya. Saat ini menggunakan field `ProductCode` langsung, akan diubah menjadi parsing dari field `Command` (format: `voucher_code*denomination_voucher_code`) untuk mengekstrak `denominationCode`.

Kedua alur menggunakan infrastruktur yang sudah ada: `consumePayload` untuk menerima pesan dari RabbitMQ, `forwardCallback` untuk mempublikasikan status akhir ke downstream via MQ_Publisher, dan UniPin client yang sudah memiliki semua API yang dibutuhkan (ValidateUser, CreateOrder, OrderInquiry, VoucherRequest, VoucherInquiry).

## Glosarium

- **Consumer**: Komponen `ConsumerServiceImpl` yang mengonsumsi pesan dari RabbitMQ dan memproses transaksi berdasarkan `productType`.
- **UniPin_Client**: HTTP client yang sudah ada untuk berkomunikasi dengan UniPin API. Memiliki method: `ValidateUser`, `CreateOrder`, `OrderInquiry`, `VoucherRequest`, `VoucherInquiry`.
- **MQ_Publisher**: Komponen yang mempublikasikan status akhir transaksi ke RabbitMQ downstream via `forwardCallback`.
- **Command**: Field baru pada `consumePayload` yang berisi string dengan format delimiter `*`. Untuk unipin-game: `gamecode*denomination_id`. Untuk unipin-voucher: `voucher_code*denomination_voucher_code`.
- **MSISDN**: Field pada `consumePayload`. Untuk unipin-voucher berisi nomor telepon biasa. Untuk unipin-game berisi JSON string (contoh: `"{\"userid\":\"123\",\"zone\":\"ID\"}"`) yang perlu di-unmarshal ke `map[string]any`.
- **Validate_User_API**: Endpoint UniPin In-Game Topup untuk memvalidasi user game. Menerima `game_code` dan `fields` (map key-value), mengembalikan `validation_token` dan `username`.
- **Create_Order_API**: Endpoint UniPin In-Game Topup untuk membuat order. Menerima `game_code`, `validation_token`, `reference_no`, dan `denomination_id`.
- **Order_Inquiry_API**: Endpoint UniPin In-Game Topup untuk mengecek status order. Digunakan sebagai fallback saat Create Order timeout.
- **Voucher_Request_API**: Endpoint UniPin Voucher untuk membeli voucher. Menerima `denomination_code`, `quantity`, dan `reference_no`.
- **Voucher_Inquiry_API**: Endpoint UniPin Voucher untuk mengecek status pembelian voucher. Digunakan sebagai fallback saat Voucher Request timeout.
- **consumePayload**: Struct yang merepresentasikan pesan dari RabbitMQ, berisi field: Amount, StockType, ProductCode, ProductID, ProductType, MID, StoreID, QueueName, MSISDN, MsgID, CallbackURL, MQTransaction, dan field baru Command.
- **forwardCallback**: Helper method pada Consumer yang mempublikasikan status transaksi (`SUCCESS`/`FAILED`) ke RabbitMQ downstream menggunakan MQ_Publisher.
- **TechnicalError**: Tipe error dari UniPin client yang mengindikasikan kegagalan teknis (network, timeout, HTTP error).
- **BusinessError**: Tipe error dari UniPin client yang mengindikasikan kegagalan bisnis (status bukan 1).

---

## Requirements

### Requirement 1: Penambahan Field Command pada consumePayload

**User Story:** Sebagai developer, saya ingin `consumePayload` memiliki field `Command` yang di-parse dari pesan RabbitMQ, sehingga alur unipin-game dan unipin-voucher dapat mengekstrak parameter transaksi dari field tersebut.

#### Acceptance Criteria

1. THE Consumer SHALL menambahkan field `Command` bertipe `string` pada struct `consumePayload`.
2. THE Consumer SHALL mem-parse field `Command` dari pesan RabbitMQ dengan mencari key `"command"` pada JSON payload (case-insensitive lookup mengikuti pola field lain di `UnmarshalJSON`).
3. THE Consumer SHALL melakukan `strings.TrimSpace` pada nilai `Command` setelah parsing, konsisten dengan pola parsing field lain di `consumePayload`.

---

### Requirement 2: Parsing Command untuk unipin-game

**User Story:** Sebagai developer, saya ingin field `Command` di-parse untuk mengekstrak `gameCode` dan `denominationID` pada alur unipin-game, sehingga parameter tersebut dapat digunakan untuk pemanggilan UniPin API.

#### Acceptance Criteria

1. WHEN `productType` adalah `"unipin-game"`, THE Consumer SHALL mem-parse field `Command` dengan delimiter `*` untuk mengekstrak dua bagian: `gameCode` (bagian pertama) dan `denominationID` (bagian kedua).
2. IF field `Command` kosong atau tidak mengandung delimiter `*`, THEN THE Consumer SHALL mencatat log error dan meneruskan status `FAILED` ke downstream via `forwardCallback`.
3. IF `gameCode` hasil parsing kosong (string kosong setelah trim), THEN THE Consumer SHALL mencatat log error dan meneruskan status `FAILED` ke downstream via `forwardCallback`.
4. IF `denominationID` hasil parsing kosong (string kosong setelah trim), THEN THE Consumer SHALL mencatat log error dan meneruskan status `FAILED` ke downstream via `forwardCallback`.

---

### Requirement 3: Parsing MSISDN sebagai JSON untuk unipin-game

**User Story:** Sebagai developer, saya ingin field `MSISDN` di-unmarshal dari JSON string menjadi `map[string]any` pada alur unipin-game, sehingga field-field validasi user (seperti `userid`, `zone`) dapat diekstrak untuk Validate User API.

#### Acceptance Criteria

1. WHEN `productType` adalah `"unipin-game"`, THE Consumer SHALL melakukan `json.Unmarshal` pada field `MSISDN` untuk menghasilkan `map[string]any` yang berisi field-field validasi user.
2. IF `MSISDN` kosong atau bukan JSON string yang valid, THEN THE Consumer SHALL mencatat log error dan meneruskan status `FAILED` ke downstream via `forwardCallback`.
3. IF hasil unmarshal `MSISDN` menghasilkan map kosong (tidak ada field), THEN THE Consumer SHALL mencatat log error dan meneruskan status `FAILED` ke downstream via `forwardCallback`.
4. THE Consumer SHALL menggunakan hasil unmarshal `MSISDN` sebagai parameter `Fields` pada `ValidateUserRequest` tanpa modifikasi key atau value.

---

### Requirement 4: Pemanggilan Validate User API untuk unipin-game

**User Story:** Sebagai operator, saya ingin user game divalidasi terlebih dahulu sebelum membuat order, sehingga hanya transaksi untuk user yang valid yang diproses.

#### Acceptance Criteria

1. WHEN `gameCode` dan `fields` (dari MSISDN) berhasil di-parse, THE Consumer SHALL memanggil `UniPin_Client.ValidateUser` dengan `ValidateUserRequest` yang berisi `GameCode` dari hasil parsing Command dan `Fields` dari hasil unmarshal MSISDN.
2. WHEN Validate_User_API mengembalikan respons sukses (`Status = 1`), THE Consumer SHALL menyimpan `ValidationToken` dari respons untuk digunakan pada Create Order API.
3. WHEN Validate_User_API mengembalikan respons gagal (`Status` bukan `1`), THE Consumer SHALL mencatat log error dan meneruskan status `FAILED` ke downstream via `forwardCallback` dengan `message` berisi `Reason` dari respons.
4. WHEN Validate_User_API mengalami error teknis (TechnicalError), THE Consumer SHALL mencatat log error dan meneruskan status `FAILED` ke downstream via `forwardCallback` dengan `message` berisi detail error.

---

### Requirement 5: Pemanggilan Create Order API untuk unipin-game

**User Story:** Sebagai operator, saya ingin order in-game topup dibuat setelah validasi user berhasil, sehingga transaksi dapat diproses oleh UniPin.

#### Acceptance Criteria

1. WHEN validasi user berhasil, THE Consumer SHALL memanggil `UniPin_Client.CreateOrder` dengan `CreateOrderRequest` yang berisi: `GameCode` dari hasil parsing Command, `ValidationToken` dari respons Validate User, `ReferenceNo` dari `msgID`, dan `DenominationID` dari hasil parsing Command.
2. WHEN Create_Order_API mengembalikan respons sukses (`Status = 1`), THE Consumer SHALL meneruskan status `SUCCESS` ke downstream via `forwardCallback` dengan `serial_number` diisi dari `ReferenceNo` respons dan `message` berisi `Reason` dari respons.
3. WHEN Create_Order_API mengembalikan respons gagal (`Status` bukan `1`), THE Consumer SHALL meneruskan status `FAILED` ke downstream via `forwardCallback` dengan `serial_number` diisi dari `ReferenceNo` respons dan `message` berisi `Reason` dari respons.
4. WHEN Create_Order_API mengalami timeout (TechnicalError dengan cause `context.DeadlineExceeded`), THE Consumer SHALL melakukan fallback ke Order Inquiry API untuk mengecek status akhir order.
5. WHEN Create_Order_API mengalami error teknis selain timeout, THE Consumer SHALL meneruskan status `FAILED` ke downstream via `forwardCallback` dengan `message` berisi detail error.

---

### Requirement 6: Fallback Order Inquiry untuk unipin-game

**User Story:** Sebagai operator, saya ingin status order dicek via Order Inquiry API saat Create Order timeout, sehingga transaksi yang sebenarnya berhasil di sisi UniPin tidak salah dilaporkan sebagai gagal.

#### Acceptance Criteria

1. WHEN Create Order timeout, THE Consumer SHALL memanggil `UniPin_Client.OrderInquiry` dengan `referenceNo` yang sama (yaitu `msgID`).
2. WHEN Order_Inquiry_API mengembalikan respons sukses (`Status = 1`), THE Consumer SHALL meneruskan status `SUCCESS` ke downstream via `forwardCallback` dengan `serial_number` diisi dari `ReferenceNo` respons inquiry dan `message` berisi `Reason` dari respons inquiry.
3. WHEN Order_Inquiry_API mengembalikan respons gagal (`Status` bukan `1`), THE Consumer SHALL meneruskan status `FAILED` ke downstream via `forwardCallback` dengan `serial_number` diisi dari `ReferenceNo` respons inquiry dan `message` berisi `Reason` dari respons inquiry.
4. WHEN Order_Inquiry_API mengalami error (TechnicalError atau error lainnya), THE Consumer SHALL meneruskan status `FAILED` ke downstream via `forwardCallback` dengan `message` berisi detail error.

---

### Requirement 7: Registrasi Product Type unipin-game di Consumer Switch

**User Story:** Sebagai developer, saya ingin product type `"unipin-game"` terdaftar di switch statement `processMessage`, sehingga pesan dengan product type tersebut diproses oleh alur yang benar.

#### Acceptance Criteria

1. THE Consumer SHALL menambahkan case `"unipin-game"` pada switch statement di method `processMessage`.
2. WHEN `productType` adalah `"unipin-game"`, THE Consumer SHALL memanggil method `processGame` (method baru) yang menangani seluruh alur: parsing Command, parsing MSISDN, Validate User, Create Order, dan fallback Order Inquiry.
3. THE Consumer SHALL menghapus atau mengganti log warning "not yet implemented" yang saat ini ditampilkan untuk product type yang belum diimplementasikan, khusus untuk `"unipin-game"`.

---

### Requirement 8: Update Parsing denominationCode untuk unipin-voucher

**User Story:** Sebagai developer, saya ingin `denominationCode` untuk alur unipin-voucher diambil dari field `Command` (bukan `ProductCode`), sehingga konsisten dengan format pengiriman data dari upstream.

#### Acceptance Criteria

1. WHEN `productType` adalah `"unipin-voucher"`, THE Consumer SHALL mem-parse field `Command` dengan delimiter `*` untuk mengekstrak dua bagian: `voucherCode` (bagian pertama) dan `denominationCode` (bagian kedua).
2. THE Consumer SHALL menggunakan `denominationCode` hasil parsing dari `Command` sebagai parameter `DenominationCode` pada `VoucherRequestReq`, menggantikan penggunaan `ProductCode` yang sebelumnya.
3. IF field `Command` kosong atau tidak mengandung delimiter `*`, THEN THE Consumer SHALL mencatat log error dan meneruskan status `FAILED` ke downstream via `forwardCallback`.
4. IF `denominationCode` hasil parsing kosong (string kosong setelah trim), THEN THE Consumer SHALL mencatat log error dan meneruskan status `FAILED` ke downstream via `forwardCallback`.
5. THE Consumer SHALL tetap mempertahankan seluruh alur voucher yang sudah ada setelah parsing: VoucherRequest → timeout fallback ke VoucherInquiry → forward ke downstream.

---

### Requirement 9: Logging Alur unipin-game

**User Story:** Sebagai operator, saya ingin setiap tahap alur unipin-game dicatat di log, sehingga saya dapat memantau dan men-debug transaksi in-game topup.

#### Acceptance Criteria

1. WHEN pesan unipin-game diterima, THE Consumer SHALL mencatat log info yang menyertakan `queue_name`, `msisdn`, `mid`, `msgid`, `game_code`, dan `denomination_id` hasil parsing.
2. WHEN Validate User API dipanggil, THE Consumer SHALL mencatat log info yang menyertakan `game_code`, `msgid`, dan jumlah fields yang dikirim.
3. WHEN Validate User API mengembalikan respons, THE Consumer SHALL mencatat log info yang menyertakan `game_code`, `msgid`, `username`, dan `status` dari respons.
4. WHEN Create Order API dipanggil, THE Consumer SHALL mencatat log info yang menyertakan `game_code`, `denomination_id`, `reference_no`, dan `msgid`.
5. WHEN Create Order API mengembalikan respons, THE Consumer SHALL mencatat log info yang menyertakan `reference_no`, `transaction_number`, `status`, dan `reason` dari respons.
6. WHEN fallback ke Order Inquiry dilakukan, THE Consumer SHALL mencatat log warn yang menyertakan `reference_no`, `msgid`, dan alasan fallback (timeout).
7. WHEN Order Inquiry API mengembalikan respons, THE Consumer SHALL mencatat log info yang menyertakan `reference_no`, `transaction_number`, `status`, dan `reason` dari respons inquiry.

---

### Requirement 10: Toleransi Kegagalan Alur unipin-game

**User Story:** Sebagai operator, saya ingin kegagalan pada satu tahap alur unipin-game tidak mengganggu pemrosesan pesan lain, sehingga consumer tetap berjalan normal.

#### Acceptance Criteria

1. IF terjadi error pada tahap apapun (parsing, Validate User, Create Order, Order Inquiry), THEN THE Consumer SHALL meneruskan status `FAILED` ke downstream dan tetap melanjutkan pemrosesan pesan berikutnya (ack pesan RabbitMQ).
2. IF `forwardCallback` gagal mempublikasikan ke RabbitMQ downstream, THEN THE Consumer SHALL mencatat error ke log dan tetap melanjutkan pemrosesan (tidak menghentikan consumer).
3. THE Consumer SHALL menggunakan context dari `processMessage` untuk setiap pemanggilan UniPin API, sehingga pembatalan context consumer juga membatalkan API call yang sedang berjalan.
4. IF `MQ_Publisher` nil, THEN THE Consumer SHALL mencatat log warning saat `forwardCallback` dipanggil dan tetap melanjutkan pemrosesan tanpa mempublikasikan pesan.
