package domain

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"nehuang-iot-server/internal/controlpolicy"
	"strings"
	"unicode"
)

var ErrRawDebugRateLimited = errors.New("raw debug rate limit")
var ErrRawDebugConflict = errors.New("raw debug request conflicts with existing command")

type RawDebugCommandInput struct {
	GatewayID                     uint64
	GatewayName                   string
	UserID                        uint64
	Role                          Role
	CommandHex, RequestID, Remark string
}

func ValidateRawDebugHex(input string) (string, error) { return validateDownlinkHex(input, false) }

// ValidateControlHex preserves only the existing F0/0F/00 custom control contract.
func ValidateControlHex(input string) (string, error) { return validateDownlinkHex(input, true) }
func validateDownlinkHex(input string, allowLegacy bool) (string, error) {
	var b strings.Builder
	for _, c := range input {
		if unicode.IsSpace(c) {
			continue
		}
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return "", fmt.Errorf("only hex digits and whitespace are accepted")
		}
		b.WriteRune(c)
	}
	text := strings.ToUpper(b.String())
	if len(text) == 0 || len(text)%2 != 0 || len(text) > 1024 {
		return "", fmt.Errorf("command must contain 1..512 complete bytes")
	}
	raw, _ := hex.DecodeString(text)
	if err := controlpolicy.ValidateDownlink(raw, allowLegacy); err != nil {
		return "", err
	}
	return text, nil
}
func IsRawDebugExtra(extra string) bool {
	var v struct {
		Source string `json:"source"`
	}
	return json.Unmarshal([]byte(extra), &v) == nil && v.Source == "admin_raw_debug"
}
