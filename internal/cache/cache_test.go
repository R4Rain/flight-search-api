package cache

import (
	"testing"
	"time"

	"github.com/service/flight-search/internal/model"
)

func TestCache_SetAndGet(t *testing.T) {
	c := New(1 * time.Second)
	flights := []model.Flight{
		{ID: "test1", FlightNumber: "GA400", Price: model.Price{Amount: 1000}},
	}
	meta := model.Metadata{TotalResults: 1}

	c.Set("key1", flights, meta)

	got, gotMeta, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(got) != 1 || got[0].ID != "test1" {
		t.Errorf("unexpected flights: %v", got)
	}
	if gotMeta.TotalResults != 1 {
		t.Errorf("unexpected metadata: %v", gotMeta)
	}
}

func TestCache_Miss(t *testing.T) {
	c := New(1 * time.Second)
	_, _, ok := c.Get("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestCache_Expiry(t *testing.T) {
	c := New(50 * time.Millisecond)
	flights := []model.Flight{{ID: "test1"}}
	meta := model.Metadata{}

	c.Set("key1", flights, meta)

	// Should be available immediately
	_, _, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected cache hit before expiry")
	}

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	_, _, ok = c.Get("key1")
	if ok {
		t.Error("expected cache miss after expiry")
	}
}

func TestCache_Key(t *testing.T) {
	req1 := model.SearchRequest{Origin: "CGK", Destination: "DPS", DepartureDate: "2025-12-15", Passengers: 1, CabinClass: "economy"}
	req2 := model.SearchRequest{Origin: "CGK", Destination: "DPS", DepartureDate: "2025-12-15", Passengers: 1, CabinClass: "economy"}
	req3 := model.SearchRequest{Origin: "CGK", Destination: "SUB", DepartureDate: "2025-12-15", Passengers: 1, CabinClass: "economy"}

	k1 := Key(req1)
	k2 := Key(req2)
	k3 := Key(req3)

	if k1 != k2 {
		t.Errorf("same requests produced different keys: %s != %s", k1, k2)
	}
	if k1 == k3 {
		t.Errorf("different requests produced same key: %s", k1)
	}
}
