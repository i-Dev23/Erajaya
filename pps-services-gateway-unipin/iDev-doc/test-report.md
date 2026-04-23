# Test Report — Game List Sync to Database

Tanggal: 8 April 2026
Branch: staging
Environment: DEV (dev-api.unipin.com)

---

## 1. GET /api/v1/ingame/list

Request:
```
GET http://localhost:8080/api/v1/ingame/list
```

Response (status 200):
```json
{
  "game_list": [
    {
      "game_category": "MLBB_ID",
      "game_code": "MLBBD_ID",
      "game_name": "Mobile Legends Diamonds",
      "icon_url": "https://storage.googleapis.com/unipin-dev/images/icon_direct_topup_games/1565343343-icon-1548659712-icon-Mobile legend 300x300 px.png",
      "game_status": "active",
      "updated_at": "2021-05-31 14:28:37",
      "product_name": "Diamonds",
      "category_name": "Mobile Legends"
    },
    {
      "game_category": "MLBB_ID",
      "game_code": "MLBBS_ID",
      "game_name": "Mobile Legends Starlight Member",
      "icon_url": "https://storage.googleapis.com/unipin-dev/images/icon_direct_topup_games/1565343258-icon-1548659712-icon-Mobile legend 300x300 px.png",
      "game_status": "active",
      "updated_at": "2019-08-09 16:34:18",
      "product_name": "Starlight Member",
      "category_name": "Mobile Legends"
    }
  ],
  "status": 1,
  "reason": "Successful"
}
```
Total game: 100+

---

## 2. GET /api/v1/ingame/detail?game_code=MLBBD_ID

Request:
```
GET http://localhost:8080/api/v1/ingame/detail?game_code=MLBBD_ID
```

Response (status 200):
```json
{
  "help_image_url": "images/help_images_direct_topup_games/1565343343-help-ml_userid_demo.png",
  "game": {
    "name": "Mobile Legends Diamonds",
    "code": "MLBBD_ID",
    "category": "MLBB_ID"
  },
  "denominations": [
    {
      "id": 1246,
      "name": "5 Diamonds + 0 Bonus",
      "currency": "IDR",
      "amount": "1915.00"
    },
    {
      "id": 318,
      "name": "11 Diamonds + 1 Bonus",
      "currency": "IDR",
      "amount": "4467.00"
    },
    {
      "id": 24,
      "name": "17 Diamonds + 2 Bonus",
      "currency": "IDR",
      "amount": "7019.00"
    },
    {
      "id": 319,
      "name": "25 Diamonds + 3 Bonus",
      "currency": "IDR",
      "amount": "10208.00"
    },
    {
      "id": 25,
      "name": "33 Diamonds + 3 Bonus",
      "currency": "IDR",
      "amount": "11100.00"
    },
    {
      "id": 320,
      "name": "53 Diamonds + 6 Bonus",
      "currency": "IDR",
      "amount": "20416.00"
    },
    {
      "id": 4317,
      "name": "Weekly Elite Bundle",
      "currency": "IDR",
      "amount": "21407.00"
    },
    {
      "id": 26,
      "name": "67 Diamonds + 7 Bonus",
      "currency": "IDR",
      "amount": "22000.00"
    },
    {
      "id": 325,
      "name": "77 Diamonds + 8 Bonus",
      "currency": "IDR",
      "amount": "29348.00"
    },
    {
      "id": 2769,
      "name": "One Time Weekly Diamond",
      "currency": "IDR",
      "amount": "37004.00"
    },
    {
      "id": 1351,
      "name": "100 Percent Diamond Rebate",
      "currency": "IDR",
      "amount": "46200.00"
    },
    {
      "id": 28,
      "name": "167 Diamonds + 18 Bonus",
      "currency": "IDR",
      "amount": "55000.00"
    },
    {
      "id": 324,
      "name": "154 Diamonds + 16 Bonus",
      "currency": "IDR",
      "amount": "58696.00"
    },
    {
      "id": 29,
      "name": "200 Diamonds + 22 Bonus",
      "currency": "IDR",
      "amount": "66000.00"
    },
    {
      "id": 1249,
      "name": "217 Diamonds + 23 Bonus",
      "currency": "IDR",
      "amount": "82767.00"
    },
    {
      "id": 3199,
      "name": "Coupon Pass",
      "currency": "IDR",
      "amount": "96640.00"
    },
    {
      "id": 321,
      "name": "256 Diamonds + 40 Bonus",
      "currency": "IDR",
      "amount": "101848.00"
    },
    {
      "id": 4316,
      "name": "Monthly Epic Bundle",
      "currency": "IDR",
      "amount": "106462.00"
    },
    {
      "id": 30,
      "name": "333 Diamonds + 37 Bonus",
      "currency": "IDR",
      "amount": "110000.00"
    },
    {
      "id": 1250,
      "name": "367 Diamonds + 41 Bonus",
      "currency": "IDR",
      "amount": "140071.00"
    },
    {
      "id": 31,
      "name": "503 Diamonds + 65 Bonus",
      "currency": "IDR",
      "amount": "191052.00"
    },
    {
      "id": 322,
      "name": "774 Diamonds + 101 Bonus",
      "currency": "IDR",
      "amount": "292843.00"
    },
    {
      "id": 33,
      "name": "1003 Diamonds + 156 Bonus",
      "currency": "IDR",
      "amount": "330000.00"
    },
    {
      "id": 34,
      "name": "1708 Diamonds + 302 Bonus",
      "currency": "IDR",
      "amount": "636608.00"
    }
  ],
  "fields": [
    {
      "name": "userid",
      "type": "string"
    },
    {
      "name": "zoneid",
      "type": "number"
    }
  ],
  "status": 1,
  "reason": "Successful"
}
```
Total denominations: 24

---

## 3. POST /api/v1/ingame/sync/MLBBD_ID/324

Request:
```
POST http://localhost:8080/api/v1/ingame/sync/MLBBD_ID/324
```

Response (status 200):
```json
{
  "denomination_id": 324,
  "game_code": "MLBBD_ID",
  "message": "denomination synced to database",
  "status": "ok"
}
```

Data yang masuk ke Oracle DB via SP MSG.PKG_UNIPIN.INSUPDGAMELIST:
```
INGAMECODE           = MLBBD_ID
INGAMEDESC           = Mobile Legends Diamonds
INGAMECATEGORY       = MLBB_ID
INGAMEDENOMINATIONID = 324
INGAMENAME           = 154 Diamonds + 16 Bonus
INGAMECURRENCY       = IDR
INGAMEAMOUNT         = 58696
INGAMEFIELDREQUEST   = [{"name":"userid","type":"string"},{"name":"zoneid","type":"number"}]
INPROVIDER           = UNIPIN
OUTERRCODE           = 0
OUTERRMSG            = (empty)
```
