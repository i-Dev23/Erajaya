// k6 Load Test untuk pps-services-database
// Menguji performa endpoint transaksi dengan banyak data dan multi-biller.
//
// Cara menjalankan:
//   k6 run test/loadtest/k6_load_test.js
//
// Dengan output ke file:
//   k6 run --out json=results.json test/loadtest/k6_load_test.js

import cryptojs from "https://cdnjs.cloudflare.com/ajax/libs/crypto-js/4.2.0/crypto-js.js";
import moment from "https://cdnjs.cloudflare.com/ajax/libs/moment.js/2.30.1/moment.min.js";
import urlencoded from "https://jslib.k6.io/form-urlencoded/3.0.0/index.js";
import { randomIntBetween, randomItem, randomString } from 'https://jslib.k6.io/k6-utils/1.5.0/index.js';
import { check, fail, group } from 'k6';
import exec from "k6/execution";
import http from 'k6/http';
import { Counter, Rate, Trend } from 'k6/metrics';

// === Custom Metrics ===
// Metric untuk tracking performa spesifik aplikasi
const transactionSuccess = new Counter('transaction_success');    // Total transaksi berhasil
const transactionFailed = new Counter('transaction_failed');      // Total transaksi gagal
const callbackSuccess = new Counter('callback_success');          // Total callback berhasil
const callbackFailed = new Counter('callback_failed');            // Total callback gagal
const publishRate = new Rate('publish_rate');                     // Rate publish berhasil
const transactionDuration = new Trend('transaction_duration');    // Durasi proses transaksi
const callbackDuration = new Trend('callback_duration');          // Durasi proses callback

// === Konfigurasi ===
const BASE_URL = __ENV.BASE_URL || 'http://localhost:3000';
const API_KEY = __ENV.API_KEY || 'pps-secret-api-key-change-me';

// Daftar biller untuk simulasi multi-biller
const BILLERS = [
    { name: 'H2H-QUEUE-SPECIAL-1', products: ['TELKOM-POSTPAID', 'TELKOM-SPEEDY', 'INDIHOME'] },
    // { name: 'H2H-QUEUE-SPECIAL-2', products: ['TELKOM-POSTPAID', 'TELKOM-SPEEDY', 'INDIHOME'] },
    // { name: 'biller-pln', products: ['PLN-PREPAID', 'PLN-POSTPAID', 'PLN-TOKEN'] },
    // { name: 'biller-telkom', products: ['TELKOM-POSTPAID', 'TELKOM-SPEEDY', 'INDIHOME'] },
    // { name: 'biller-bpjs', products: ['BPJS-KESEHATAN', 'BPJS-KETENAGAKERJAAN'] },
    // { name: 'biller-pdam', products: ['PDAM-JAKARTA', 'PDAM-BANDUNG', 'PDAM-SURABAYA'] },
    // { name: 'biller-pgn', products: ['PGN-GAS', 'PGN-NONTAGLIS'] },
    // { name: 'biller-multifinance', products: ['FIF', 'ADIRA', 'WOM'] },
    // { name: 'biller-insurance', products: ['PRUDENTIAL', 'ALLIANZ', 'AXA'] },
    // { name: 'biller-ecommerce', products: ['TOKOPEDIA', 'SHOPEE', 'BUKALAPAK'] },
    // { name: 'biller-tv', products: ['INDOVISION', 'TRANSVISION', 'TOPAS-TV'] },
    // { name: 'biller-internet', products: ['BIZNET', 'MYREPUBLIC', 'FIRSTMEDIA'] },
];

// Response codes yang mungkin dari biller
const RESPONSE_CODES = ['00', '01', '14', '68', '96'];

// === Skenario Load Test ===
// Menggunakan stages untuk simulasi traffic pattern realistis:
// 1. Ramp-up: naikkan load secara bertahap
// 2. Steady state: pertahankan load konstan
// 3. Spike: simulasi lonjakan traffic
// 4. Recovery: turunkan load
// 5. Ramp-down: turunkan ke nol
export const options = {
    // vus: 1,
    // iterations: 1,
    // stages: [
    //     // { duration: '30s', target: 10 },   // Ramp-up: 0 -> 10 VUs dalam 30 detik
    //     // { duration: '1m', target: 50 },     // Steady: naikkan ke 50 VUs
    //     // { duration: '2m', target: 50 },     // Steady state: pertahankan 50 VUs selama 2 menit
    //     // { duration: '30s', target: 100 },   // Spike: naikkan ke 100 VUs
    //     // { duration: '1m', target: 100 },    // Spike sustained: pertahankan 100 VUs
    //     // { duration: '30s', target: 50 },    // Recovery: turunkan ke 50 VUs
    //     // { duration: '1m', target: 50 },     // Steady: pertahankan 50 VUs
    //     // { duration: '30s', target: 0 },     // Ramp-down: turunkan ke 0
    // ],
    thresholds: {
        http_req_duration: ['p(95)<2000'],       // 95% request harus < 2 detik
        http_req_failed: ['rate<0.05'],           // Error rate harus < 5%
        publish_rate: ['rate>0.90'],              // Publish rate harus > 90%
        transaction_duration: ['p(95)<1500'],     // 95% transaksi harus < 1.5 detik
    },
};

const list = [
    {
        product: "SP10",
        nominal: "10000"
    },
    {
        product: "TS11",
        nominal: "11000"
    },
    {
        product: "TS15",
        nominal: "15000"
    },
];

// === Helper Functions ===

// generateTransaction membuat satu transaksi random dari biller tertentu.
function generateTransaction(biller) {
    const product = biller.products[randomIntBetween(0, biller.products.length - 1)];
    const rc = RESPONSE_CODES[randomIntBetween(0, RESPONSE_CODES.length - 1)];

    return {
        id: `TXN-${biller.name}-${randomString(8)}`,
        queue_name: biller.name,
        rc: rc,
        product: product,
        client_number: `${randomIntBetween(10000000000, 99999999999)}`,
        message: `Transaction ${rc === '00' ? 'successful' : 'pending'} for ${product}`,
        serial_number: rc === '00' ? `SN-${randomString(10)}` : '',
        nominal: `${randomIntBetween(10000, 1000000)}`,
        additional_message: `Ref: ${randomString(12)}`,
    };
}

// generateBatchTransactions membuat batch transaksi dari multiple biller.
function generateBatchTransactions(count) {
    const transactions = [];
    for (let i = 0; i < count; i++) {
        const biller = BILLERS[randomIntBetween(0, BILLERS.length - 1)];
        transactions.push(generateTransaction(biller));
    }
    return { transactions };
}

// // generateCallback membuat callback request.
// function generateCallback(transactionId) {
//     return {
//         id: transactionId,
//         conversation_id: `CONV-${randomString(8)}`,
//         original_conversation_id: `ORIG-${randomString(8)}`,
//         status_to_be: 'SUCCESS',
//         message_to_customer: 'Payment has been processed successfully',
//         additional_message: `Callback ref: ${randomString(12)}`,
//     };
// }

// generateCallback membuat callback request.
function generateCallback(msgId, clientNumber, nominal, queueName) {
    const reqBody = {
        source: "PROVIDER",
        data: {
            msgId: msgId,
            statusToBe: randomString(1, "FC"),
            serialNumber: `SN-${randomString(10)}`,
            clientNumber: clientNumber,
            nominal: nominal,
            originalConversationId: `${msgId}`,
            conversationId: `${msgId}`,
            additionalMessage: `no ref <${randomString(12)}>`,
            queueName: queueName,
        }
    };
    if (reqBody.data.statusToBe == "F") {
        reqBody.data.messageToCustomer = `Pengisian Voucher sebesar ${nominal} ke nomor ${clientNumber} pada tanggal ${moment().format("DD/MM/YYYY HH:mm:ss")} telah berhasil dengan ${reqBody.data.additionalMessage}`;
    } else {
        reqBody.data.messageToCustomer = `Pengisian Voucher sebesar ${nominal} ke nomor ${clientNumber} pada tanggal ${moment().format("DD/MM/YYYY HH:mm:ss")} GAGAL`;
    }

    return reqBody;
}

// getHeaders mengembalikan headers standar untuk request.
function getHeaders() {
    return {
        'Content-Type': 'application/json',
        'X-API-Key': API_KEY,
    };
}

function zeroPad(num, places) {
    const zero = places - num.toString().length + 1;

    return Array(+(zero > 0 && zero)).join("0") + num;
}

function sell(uniqPrefix) {
    const generateId = uniqPrefix + zeroPad(exec.vu.idInTest, 3) + zeroPad(exec.scenario.iterationInTest + 1, 4);
    // console.log("sell", "generateId: " + generateId)
    const randomData = randomItem(list);
    let bodyReq = {
        user: "UT",
        produk: randomData.product,
        mdn: `6281${randomIntBetween(100000000, 999999999)}`,
        notrx: generateId,
        signature: ""
    };
    bodyReq.signature = cryptojs.enc.Hex.stringify(cryptojs.MD5(bodyReq.mdn + bodyReq.produk + bodyReq.notrx + "e807f1fcf82d132f9bb018ca6738a19f"));
    const response = http.post("https://paymentservices-evs.erajaya.com:9447/h2h-queue-devl/Sell", urlencoded(bodyReq), {
        headers: {
            "Content-Type": "application/x-www-form-urlencoded",
        },
        timeout: '180s',
    });
    console.log("sell", "||", bodyReq, "||", response.json());
    if (!check(response, {
        "[S] status code MUST be 200": (res) => res.status == 200,
    })) {
        fail("[S] status code was not 200 or outError MUST was not 0");
    }

    return [bodyReq, response.json(), randomData.nominal];
}

// === Main Test Function ===
// Setiap VU (Virtual User) menjalankan fungsi ini secara berulang.
export default function (data) {
    // // Group 1: Submit Batch Transactions (multi-biller)
    // group('Submit Batch Transactions', function () {
    //     // Variasi ukuran batch: 1, 5, 10, 20 transaksi
    //     const batchSizes = [1, 5, 10, 20];
    //     const batchSize = batchSizes[randomIntBetween(0, batchSizes.length - 1)];

    //     const payload = generateBatchTransactions(batchSize);

    //     const startTime = Date.now();
    //     const res = http.post(
    //         `${BASE_URL}/api/transactions`,
    //         JSON.stringify(payload),
    //         { headers: getHeaders(), tags: { name: 'SubmitTransactions' } }
    //     );
    //     const duration = Date.now() - startTime;
    //     transactionDuration.add(duration);

    //     const success = check(res, {
    //         'status is 200': (r) => r.status === 200,
    //         'response has data': (r) => {
    //             try {
    //                 const body = JSON.parse(r.body);
    //                 return body.data !== undefined;
    //             } catch (e) {
    //                 return false;
    //             }
    //         },
    //         'all transactions published': (r) => {
    //             try {
    //                 const body = JSON.parse(r.body);
    //                 return body.data && body.data.failed === 0;
    //             } catch (e) {
    //                 return false;
    //             }
    //         },
    //     });

    //     if (success) {
    //         transactionSuccess.add(batchSize);
    //         publishRate.add(true);
    //     } else {
    //         transactionFailed.add(batchSize);
    //         publishRate.add(false);
    //     }
    // });

    // sleep(randomIntBetween(1, 3) / 10); // 0.1 - 0.3 detik jeda

    // // Group 2: Submit Single Transaction per Biller
    // group('Submit Single Transaction Per Biller', function () {
    //     const biller = BILLERS[randomIntBetween(0, BILLERS.length - 1)];
    //     const payload = {
    //         transactions: [generateTransaction(biller)],
    //     };

    //     const res = http.post(
    //         `${BASE_URL}/api/transactions`,
    //         JSON.stringify(payload),
    //         { headers: getHeaders(), tags: { name: 'SingleTransaction' } }
    //     );

    //     console.log(res.json());

    //     check(res, {
    //         'single tx status 200': (r) => r.status === 200,
    //     });
    // });

    // sleep(randomIntBetween(1, 3) / 10);

    // Group 3: Submit Callback
    group('Submit Callback', function () {
        const [bodyReq, bodyResp, nominal] = sell(data.uniqPrefix);

        const biller = BILLERS[randomIntBetween(0, BILLERS.length - 1)];
        // const txId = `TXN-${biller.name}-${randomString(8)}`;
        const payload = generateCallback(bodyResp.ServerIDTrx, bodyReq.mdn, nominal, biller.name);
        console.log(bodyResp.ServerIDTrx);

        const startTime = Date.now();
        const res = http.post(
            `${BASE_URL}/api/callback/topup`,
            JSON.stringify(payload),
            { headers: getHeaders(), tags: { name: 'SubmitCallback' } }
        );

        console.log(res);

        const duration = Date.now() - startTime;
        callbackDuration.add(duration);

        const success = check(res, {
            'callback status 200 or 404': (r) => r.status === 200 || r.status === 404,
        });

        if (success && res.status === 200) {
            callbackSuccess.add(1);
        } else {
            callbackFailed.add(1);
        }
    });

    // sleep(randomIntBetween(1, 3) / 10);

    // // Group 4: Health Check
    // group('Health Check', function () {
    //     const res = http.get(`${BASE_URL}/health`, {
    //         tags: { name: 'HealthCheck' },
    //     });

    //     check(res, {
    //         'health status 200': (r) => r.status === 200,
    //         'health is healthy': (r) => {
    //             try {
    //                 const body = JSON.parse(r.body);
    //                 return body.data && (body.data.status === 'healthy' || body.data.status === 'degraded');
    //             } catch (e) {
    //                 return false;
    //             }
    //         },
    //     });
    // });

    // sleep(randomIntBetween(1, 5) / 10);

    // // Group 5: Stress Test - Large Batch (50 transactions)
    // group('Large Batch Stress Test', function () {
    //     // Hanya jalankan 10% dari waktu untuk tidak overload
    //     if (Math.random() < 0.1) {
    //         const payload = generateBatchTransactions(50);

    //         const res = http.post(
    //             `${BASE_URL}/api/transactions`,
    //             JSON.stringify(payload),
    //             { headers: getHeaders(), tags: { name: 'LargeBatch' } }
    //         );

    //         check(res, {
    //             'large batch status 200': (r) => r.status === 200,
    //             'large batch response time < 5s': (r) => r.timings.duration < 5000,
    //         });
    //     }
    // });

    // // Group 6: Negative Tests
    // group('Negative Tests', function () {
    //     // Hanya jalankan 5% dari waktu
    //     if (Math.random() < 0.05) {
    //         // Test tanpa API key
    //         const res1 = http.post(
    //             `${BASE_URL}/api/transactions`,
    //             JSON.stringify({ transactions: [] }),
    //             { headers: { 'Content-Type': 'application/json' }, tags: { name: 'NoAPIKey' } }
    //         );
    //         check(res1, {
    //             'no api key returns 401': (r) => r.status === 401,
    //         });

    //         // Test dengan invalid JSON
    //         const res2 = http.post(
    //             `${BASE_URL}/api/transactions`,
    //             'invalid json',
    //             { headers: getHeaders(), tags: { name: 'InvalidJSON' } }
    //         );
    //         check(res2, {
    //             'invalid json returns 400': (r) => r.status === 400,
    //         });

    //         // Test dengan wrong API key
    //         const res3 = http.post(
    //             `${BASE_URL}/api/transactions`,
    //             JSON.stringify({ transactions: [] }),
    //             {
    //                 headers: { 'Content-Type': 'application/json', 'X-API-Key': 'wrong-key' },
    //                 tags: { name: 'WrongAPIKey' },
    //             }
    //         );
    //         check(res3, {
    //             'wrong api key returns 401': (r) => r.status === 401,
    //         });
    //     }
    // });
}

// === Lifecycle Hooks ===

// setup dijalankan sekali sebelum test dimulai.
// Digunakan untuk verifikasi bahwa server bisa diakses.
export function setup() {
    const res = http.get(`${BASE_URL}/health`);
    if (res.status !== 200) {
        console.warn(`Health check failed with status ${res.status}. Server might not be ready.`);
    } else {
        console.log('Server is healthy, starting load test...');
    }
    return { uniqPrefix: moment().format("YYYYMMDDHHmmss"), startTime: Date.now() };
}

// teardown dijalankan sekali setelah test selesai.
// Menampilkan ringkasan hasil test.
export function teardown(data) {
    const duration = (Date.now() - data.startTime) / 1000;
    console.log(`Load test completed in ${duration.toFixed(1)} seconds`);
}
