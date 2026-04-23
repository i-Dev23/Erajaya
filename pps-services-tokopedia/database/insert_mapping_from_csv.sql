-- Insert error message mappings from CSV data (40 records total)
-- Safe to re-run: uses ON CONFLICT DO NOTHING to avoid duplicates

-- Clear existing data (optional - comment out if you want to keep existing data)
-- TRUNCATE TABLE "mapping".error_message_mappings RESTART IDENTITY CASCADE;

-- Insert Oracle mappings (21 records including duplicates)
INSERT INTO "mapping".error_message_mappings (system_type, error_pattern, response_code, is_exact_match, description)
VALUES
    -- Row 1
    ('oracle', 'H2H - Error : ''||vCutOffErrMessage', '61', FALSE, 'Server maintenance'),
    -- Row 2
    ('oracle', 'Error 02 : Signature tidak benar, komponen tidak syah.', '32', TRUE, 'Invalid signature'),
    -- Row 3
    ('oracle', 'Kode voucher is not found', '14', TRUE, 'Ineligible Product'),
    -- Row 4
    ('oracle', 'Error 01 : Signature tidak benar.', '32', TRUE, 'Invalid signature'),
    -- Row 5
    ('oracle', 'Sell Error 02 : IP belum di setting.', '60', TRUE, 'Access is not allowed'),
    -- Row 6
    ('oracle', 'Sell Error 02 : IP Anda ''|| p_ip ||'' tidak sama dengan Settingan ', '60', FALSE, 'Access is not allowed (dynamic IP)'),
    -- Row 7
    ('oracle', 'Sell Error 02 : queue setting is not ready', '60', TRUE, 'Access is not allowed'),
    -- Row 8
    ('oracle', 'Anda tidak diperkenankan transaksi lewat WEB, hubungi admin kami untuk merubah profile anda.', '60', TRUE, 'Access is not allowed'),
    -- Row 9
    ('oracle', 'Kode Voucher ''||KDV||'' tidak tersedia untuk Account Anda.', '14', FALSE, 'Ineligible Product (dynamic voucher)'),
    -- Row 10
    ('oracle', '4. Harga jual belum di setting untuk anda.', '44', TRUE, 'Invalid transaction amount'),
    -- Row 11
    ('oracle', 'Deposit anda kurang untuk memenuhi penjualan.', '43', TRUE, 'Insufficient balance'),
    -- Row 12
    ('oracle', 'Stock Voucher ''||KDV||'' habis, transaksi kami batalkan.', '50', FALSE, 'Product out of stock'),
    -- Row 13
    ('oracle', 'Fee Amount untuk ''||KDV||'' belum disetting', '44', FALSE, 'Invalid transaction amount'),
    -- Row 14-21: Duplicates from CSV (will be skipped by ON CONFLICT)
    ('oracle', 'H2H - Error : ''||vCutOffErrMessage', '61', FALSE, 'Server maintenance'),
    ('oracle', 'Error 02 : Signature tidak benar, komponen tidak syah.', '32', TRUE, 'Invalid signature'),
    ('oracle', 'Kode voucher is not found', '14', TRUE, 'Ineligible Product'),
    ('oracle', 'Error 01 : Signature tidak benar.', '32', TRUE, 'Invalid signature'),
    ('oracle', 'Sell Error 02 : IP belum di setting.', '60', TRUE, 'Access is not allowed'),
    ('oracle', 'Sell Error 02 : IP Anda ''|| p_ip ||'' tidak sama dengan Settingan ', '60', FALSE, 'Access is not allowed (dynamic IP)'),
    -- Row 20
    ('oracle', 'No Transaksi ''||p_notrx||'' tidak ditemukan.', '12', FALSE, 'Transaction not found'),
    -- Row 21
    ('oracle', 'Kode voucher is not found', '14', TRUE, 'Ineligible Product')
ON CONFLICT (system_type, error_pattern) DO NOTHING;

-- Insert Ultima mappings
INSERT INTO "mapping".error_message_mappings (system_type, error_pattern, response_code, is_exact_match, description)
VALUES
    ('ultima', 'cut-off', '63', FALSE, 'Biller maintenance'),
    ('ultima', 'yang anda masukkan salah', '20', FALSE, 'Unregistered number'),
    ('ultima', 'kwh melebihi batas maksimum', '23', FALSE, 'Limit exceeded'),
    ('ultima', 'no payment', '12', FALSE, 'Transaction not found / No payment'),
    ('ultima', 'failed system (timeout)', '41', FALSE, 'Biller timeout'),
    ('ultima', 'sistem sedang kendala', '64', FALSE, 'Biller error'),
    ('ultima', '[0024] produk dan nomor tujuan tidak sesuai', '20', TRUE, 'Unregistered number'),
    ('ultima', '[0070] produk CutOff', '61', TRUE, 'Server maintenance'),
    ('ultima', '[0070] system CutOff', '63', TRUE, 'Biller maintenance'),
    ('ultima', '[0072] Produk gangguan', '51', TRUE, 'Product closed'),
    ('ultima', 'panjang nomor tujuan tidak sesuai', '20', FALSE, 'Unregistered number'),
    ('ultima', 'TRX GAGAL:IDPEL YANG ANDA MASUKKAN SALAH, MOHON TELITI KEMBALI', '20', TRUE, 'Unregistered number'),
    ('ultima', 'TRX GAGAL:NOMOR METER YANG ANDA MASUKKAN SALAH, MOHON TELITI KEMBALI', '20', TRUE, 'Unregistered number'),
    ('ultima', 'TRX GAGAL:NOMOR METER/IDPEL YANG ANDA MASUKKAN SALAH, MOHON TELITI KEMBALI', '20', TRUE, 'Unregistered number'),
    ('ultima', 'GANGGUAN OPERATOR / SISTEM SEDANG KENDALA', '64', TRUE, 'Biller error'),
    ('ultima', 'DIBLOKIR HUBUNGI PLN', '21', TRUE, 'Number blocked'),
    ('ultima', 'TRX GAGAL:MLPO INTERNAL SYSTEM ERROR', '64', TRUE, 'Biller error'),
    ('ultima', 'TRX GAGAL:NOT ENOUGH CUSTOMER ACCOUNT BALANCE', '43', TRUE, 'Insufficient balance'),
    ('ultima', 'TRX GAGAL:SERVICE PROVIDER INTERNAL SYSTEM ERROR', '64', TRUE, 'Biller error')
ON CONFLICT (system_type, error_pattern) DO NOTHING;

-- Verify inserted data
SELECT 
    system_type, 
    COUNT(*) as total_mappings,
    COUNT(DISTINCT response_code) as unique_response_codes
FROM "mapping".error_message_mappings
GROUP BY system_type
ORDER BY system_type;

-- Show all mappings
SELECT 
    id,
    system_type,
    error_pattern,
    response_code,
    is_exact_match,
    description,
    created_at
FROM "mapping".error_message_mappings
ORDER BY system_type, response_code, error_pattern;
