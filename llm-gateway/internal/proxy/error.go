package proxy

import (
	"errors"
	"net"

	"llm_gateway/internal/storage"
)

func IsRetryableError(err error, settings storage.Settings) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return settings.EnableRetryOnTimeout
		}
		return true
	}
	return false
}

func ClassifyErrorAndGetWeight(err error, settings storage.Settings) int {
	if err == nil {
		return 0
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		var opErr *net.OpError
		if errors.As(err, &opErr) && opErr.Op == "dial" && netErr.Timeout() {
			return settings.ConnectTimeoutWeight
		}
		if netErr.Timeout() {
			return settings.TimeoutWeight
		}
		if netErr.Temporary() {
			if settings.TimeoutWeight > 1 {
				return settings.TimeoutWeight / 2
			}
			return 1
		}
		return settings.Weight5xx
	}
	return settings.Weight5xx
}
