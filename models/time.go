package models

import (
	"encoding/json"
	"math"
	"time"
)

const FM float64 = 1e9

type FTime struct {
	time.Time
}

func (p *FTime) UnmarshalJSON(data []byte) error {
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	sec, dec := math.Modf(f)
	p.Time = time.Unix(int64(sec), int64(dec*FM))
	return nil
}
