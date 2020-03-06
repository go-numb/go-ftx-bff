package models

type Board struct {
	Time     FTime       `json:"time"`
	Checksum int64       `json:"checksum"`
	Bids     [][]float64 `json:"bids"`
	Asks     [][]float64 `json:"asks"`
	Action   string      `json:"action"`
}
