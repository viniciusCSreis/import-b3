package model

import "time"

type Negotiations struct {
	NegotiationDate time.Time
	NegotiationType string
	Code            string
	Amount          int64
	Price           float64
}
