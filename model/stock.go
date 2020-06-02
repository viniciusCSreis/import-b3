package model

import "time"

type Stock struct {
	StockTime time.Time
	Price float64
	Name string
}
