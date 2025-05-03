// package scanner
package tickets

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	_ "net/http/pprof"
	"strconv"
	"strings"
	"time"
)

const (
	// hmacSecret   = "your-very-secret-key"
	allowedDrift = 5 * 60 // seconds = 5 minutes
)

// Verifies QR payload: eventID|ticketID|uniqueCode|timestamp|HMAC
func VerifyTicketQR(payload string) (eventID, ticketID, uniqueCode string, err error) {
	parts := strings.Split(payload, "|")
	if len(parts) != 5 {
		return "", "", "", errors.New("invalid QR format")
	}

	eventID = parts[0]
	ticketID = parts[1]
	uniqueCode = parts[2]
	timestampStr := parts[3]
	signature := parts[4]

	// Check timestamp window
	ts, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return "", "", "", errors.New("invalid timestamp")
	}

	now := time.Now().Unix()
	if abs(now-ts) > allowedDrift {
		return "", "", "", errors.New("ticket expired or from the future")
	}

	// Recompute signature
	data := fmt.Sprintf("%s|%s|%s|%s", eventID, ticketID, uniqueCode, timestampStr)
	h := hmac.New(sha256.New, []byte(hmacSecret))
	h.Write([]byte(data))
	expectedSig := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return "", "", "", errors.New("invalid signature")
	}

	return eventID, ticketID, uniqueCode, nil
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
