package usecase

import (
	"context"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
	"strconv"
	"strings"
	"time"
)

type cutOffWindow struct {
	startGeneral   string
	durGeneral     string
	startTokopedia string
	durTokopedia   string
}

func validateCutOffRepo(ctx context.Context, cutOffRepo domain.CutOffRepository, logger service.Logger, isActive func(string) bool, logSuffix string) (bool, error) {
	if cutOffRepo == nil {
		return false, nil
	}

	cutOffResp, errCutOff := cutOffRepo.CutOff(ctx, domain.CutOffFlagH2H)
	if errCutOff != nil {
		logger.Error("Failed to validate cut off"+logSuffix, "error", errCutOff)
		return false, errCutOff
	}

	if isActive(cutOffResp.OutErrCode) {
		logger.Error("Cut off is active"+logSuffix, "cutOffResp", cutOffResp)
		return true, nil
	}

	return false, nil
}

func validateCutOffRedisWithFallback(ctx context.Context, redisClient service.RedisClient, productRepo domain.ProductRepository, logger service.Logger, logSuffix string) bool {
	window := getCutOffWindowFromRedis(ctx, redisClient)

	if window.startGeneral == "" && window.startTokopedia == "" {
		logger.Info("Cut-off data not found in Redis, falling back to Oracle" + logSuffix)
		oracleWindow, ok := getCutOffWindowFromOracle(ctx, productRepo, redisClient, logger, logSuffix)
		if !ok {
			return false
		}
		window = oracleWindow
		return isCutOffActive(window, logger, "Oracle", logSuffix)
	}

	return isCutOffActive(window, logger, "redis", logSuffix)
}

func getCutOffWindowFromRedis(ctx context.Context, redisClient service.RedisClient) cutOffWindow {
	get := func(key string) string {
		cmd := redisClient.Get(ctx, key)
		if cmd == nil || cmd.Err() != nil {
			return ""
		}
		return cmd.Val()
	}

	return cutOffWindow{
		startGeneral:   strings.TrimSpace(get(utils.RedisKeyCutOffTimeStart)),
		durGeneral:     strings.TrimSpace(get(utils.RedisKeyCutOffDurationSecond)),
		startTokopedia: strings.TrimSpace(get(utils.RedisKeyCutOffTimeStartTokopedia)),
		durTokopedia:   strings.TrimSpace(get(utils.RedisKeyCutOffDurationSecondTokopedia)),
	}
}

func getCutOffWindowFromOracle(ctx context.Context, productRepo domain.ProductRepository, redisClient service.RedisClient, logger service.Logger, logSuffix string) (cutOffWindow, bool) {
	logger.Info("Fetching cut-off configuration from Oracle" + logSuffix)

	cutOffResponse, err := productRepo.GetCutOff(ctx)
	if err != nil {
		logger.Error("Failed to get cut-off from Oracle"+logSuffix, "error", err)
		return cutOffWindow{}, false
	}

	if cutOffResponse.OutErrCode != "0" {
		logger.Warn("Oracle returned invalid cut-off data"+logSuffix,
			"outerrcode", cutOffResponse.OutErrCode,
			"outerrmsg", cutOffResponse.OutErrMsg)
		return cutOffWindow{}, false
	}

	logger.Info("Successfully fetched cut-off from Oracle"+logSuffix,
		"cut_off_time_start", cutOffResponse.CutOffTimeStart,
		"cut_off_duration", cutOffResponse.CutOffDuration,
		"cut_off_time_start_tokopedia", cutOffResponse.CutOffTimeStartTokopedia,
		"cut_off_duration_tokopedia", cutOffResponse.CutOffDurationTokopedia)

	go func() {
		saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		mappings := map[string]string{
			utils.RedisKeyCutOffTimeStartTokopedia:      cutOffResponse.CutOffTimeStartTokopedia,
			utils.RedisKeyCutOffDurationSecondTokopedia: cutOffResponse.CutOffDurationTokopedia,
			utils.RedisKeyCutOffMessageTokopedia:        cutOffResponse.CutOffMessageTokopedia,
			utils.RedisKeyCutOffTimeStart:               cutOffResponse.CutOffTimeStart,
			utils.RedisKeyCutOffDurationSecond:          cutOffResponse.CutOffDuration,
			utils.RedisKeyCutOffMessage:                 cutOffResponse.CutOffMessage,
		}

		for key, value := range mappings {
			if value != "" {
				err := redisClient.Set(saveCtx, key, value, 0).Err()
				if err != nil {
					logger.Error("Failed to save cut-off to Redis cache (async)"+logSuffix, "error", err, "key", key)
				} else {
					logger.Info("Successfully cached cut-off to Redis (async)"+logSuffix, "key", key)
				}
			}
		}
	}()

	return cutOffWindow{
		startGeneral:   strings.TrimSpace(cutOffResponse.CutOffTimeStart),
		durGeneral:     strings.TrimSpace(cutOffResponse.CutOffDuration),
		startTokopedia: strings.TrimSpace(cutOffResponse.CutOffTimeStartTokopedia),
		durTokopedia:   strings.TrimSpace(cutOffResponse.CutOffDurationTokopedia),
	}, true
}

func isCutOffActive(window cutOffWindow, logger service.Logger, source string, logSuffix string) bool {
	if window.startGeneral == "" && window.startTokopedia == "" {
		return false
	}

	now := time.Now()
	inGeneral := withinCutOffWindow(window.startGeneral, window.durGeneral, now)
	inTokopedia := withinCutOffWindow(window.startTokopedia, window.durTokopedia, now)

	if inGeneral || inTokopedia {
		logger.Info("Cut off active from "+source+" window"+logSuffix,
			"inGeneral", inGeneral,
			"inTokopedia", inTokopedia,
			"startGeneral", window.startGeneral,
			"durGeneral", window.durGeneral,
			"startTokopedia", window.startTokopedia,
			"durTokopedia", window.durTokopedia)
		return true
	}

	return false
}

func withinCutOffWindow(startHHMM, durSec string, now time.Time) bool {
	if startHHMM == "" || durSec == "" {
		return false
	}

	t, err := time.Parse("15:04", startHHMM)
	if err != nil {
		return false
	}

	ds, err := strconv.Atoi(durSec)
	if err != nil || ds <= 0 {
		return false
	}

	start := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	end := start.Add(time.Duration(ds) * time.Second)

	if end.Before(start) {
		end = end.Add(24 * time.Hour)
		if now.Before(start) {
			nowShift := now.Add(24 * time.Hour)
			return !nowShift.Before(start) && !nowShift.After(end)
		}
	}

	return !now.Before(start) && !now.After(end)
}
