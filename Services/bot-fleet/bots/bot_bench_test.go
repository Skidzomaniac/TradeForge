package bots

import (
	"testing"
	"time"
)

func BenchmarkShadowBook_AddOrder(b *testing.B) {
	book := NewShadowBook()
	for i := 0; i < 1000; i++ {
		book.AddOrder(Order{OrderID: "x", Type: "LIMIT_SELL", Price: 100 + float64(i%50), Quantity: 1, Time: time.Now()})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		book.AddOrder(Order{OrderID: "y", Type: "MARKET_BUY", Quantity: 1, Time: time.Now()})
	}
}

func BenchmarkGenerateOrderID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = generateOrderID()
	}
}

func BenchmarkValidateFill(b *testing.B) {
	exp := ExpectedFill{Status: "FILLED", Price: 100, Quantity: 10}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ValidateFill(exp, "FILLED", 100, 10)
	}
}
