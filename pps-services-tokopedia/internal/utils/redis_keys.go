package utils

// Redis key constants for consistency (values unchanged).
const (
	RedisKeyWhitelistedIP = "WHITELISTED_IP"

	RedisKeyCutOffTimeStart              = "CUT_OFF_TIME_START"
	RedisKeyCutOffDurationSecond         = "CUT_OFF_DURATION_SECOND"
	RedisKeyCutOffMessage                = "CUT_OFF_MESSAGE"
	RedisKeyCutOffTimeStartTokopedia     = "CUT_OFF_TIME_START_TOKOPEDIA"
	RedisKeyCutOffDurationSecondTokopedia = "CUT_OFF_DURATION_SECOND_TOKOPEDIA"
	RedisKeyCutOffMessageTokopedia        = "CUT_OFF_MESSAGE_TOKOPEDIA"

	RedisKeyProductWithStatusPrefix = "product_with_status:"
	RedisKeyRateLimitPrefix         = "rate_limit:"
)
