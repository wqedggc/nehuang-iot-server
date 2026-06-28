package domain

import (
	"strconv"
)

// SnowflakeID is a uint64 that serialises as a JSON string.
// This prevents JavaScript Number precision loss for IDs > 2^53.
type SnowflakeID uint64

// MarshalJSON serialises SnowflakeID as a JSON string.
func (id SnowflakeID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.FormatUint(uint64(id), 10) + `"`), nil
}

// UnmarshalJSON deserialises a JSON string back to SnowflakeID.
func (id *SnowflakeID) UnmarshalJSON(data []byte) error {
	s := string(data)
	// Strip quotes
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return err
	}
	*id = SnowflakeID(v)
	return nil
}
