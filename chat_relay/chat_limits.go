package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const (
	maxEnvelopePayloadBytes = 128 * 1024
	maxEnvelopeWrappedKeys  = 512
	chatRateWindow          = 10 * time.Second
	chatRateMaxPerWindow    = 40
)

var (
	chatRateMu       sync.Mutex
	chatRateByClient = make(map[string][]time.Time)
)

func allowChatMessage(clientUUID string, now time.Time) bool {
	if clientUUID == "" {
		return false
	}
	chatRateMu.Lock()
	defer chatRateMu.Unlock()

	windowStart := now.Add(-chatRateWindow)
	events := chatRateByClient[clientUUID]
	trimmed := events[:0]
	for _, ts := range events {
		if ts.After(windowStart) {
			trimmed = append(trimmed, ts)
		}
	}
	if len(trimmed) >= chatRateMaxPerWindow {
		chatRateByClient[clientUUID] = append([]time.Time(nil), trimmed...)
		return false
	}
	trimmed = append(trimmed, now)
	chatRateByClient[clientUUID] = append([]time.Time(nil), trimmed...)
	return true
}

func clearChatMessageLimiter(clientUUID string) {
	if clientUUID == "" {
		return
	}
	chatRateMu.Lock()
	delete(chatRateByClient, clientUUID)
	chatRateMu.Unlock()
}

func validateEnvelopeForRelay(envelope map[string]interface{}) error {
	if len(envelope) == 0 {
		return fmt.Errorf("missing encrypted envelope")
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("invalid encrypted envelope")
	}
	if len(raw) > maxEnvelopePayloadBytes {
		return fmt.Errorf("encrypted envelope exceeds %d bytes", maxEnvelopePayloadBytes)
	}
	if wrappedRaw, ok := envelope["wrapped_keys"]; ok {
		wrappedKeys, ok := wrappedRaw.([]interface{})
		if !ok {
			return fmt.Errorf("invalid wrapped_keys format")
		}
		if len(wrappedKeys) == 0 || len(wrappedKeys) > maxEnvelopeWrappedKeys {
			return fmt.Errorf("invalid wrapped_keys count")
		}
	}
	return nil
}
