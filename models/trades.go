package models

import (
	"time"
)

type Trade struct {
	ID            int       `json:"id"`
	IsLiquidation bool      `json:"liquidation"`
	Price         float64   `json:"price"`
	Size          float64   `json:"size"`
	Side          string    `json:"side"`
	Time          time.Time `json:"time"`
}
