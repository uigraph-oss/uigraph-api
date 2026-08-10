package enterprise

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_SeatLimit(t *testing.T) {
	t.Run("decodes seatLimit and planId", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("X-Internal-Token"); got != "secret" {
				t.Errorf("X-Internal-Token = %q, want %q", got, "secret")
			}
			if r.URL.Path != "/internal/v1/orgs/org1/seat-limit" {
				t.Errorf("path = %q", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"seatLimit":25,"planId":"enterprise"}`))
		}))
		defer srv.Close()

		c := New(srv.URL, "secret")
		got, err := c.SeatLimit(context.Background(), "org1")
		if err != nil {
			t.Fatalf("SeatLimit: %v", err)
		}
		if got != (SeatLimitInfo{Limit: 25, PlanID: "enterprise"}) {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("decodes a free org with no planId", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"seatLimit":3,"planId":""}`))
		}))
		defer srv.Close()

		c := New(srv.URL, "secret")
		got, err := c.SeatLimit(context.Background(), "org1")
		if err != nil {
			t.Fatalf("SeatLimit: %v", err)
		}
		if got != (SeatLimitInfo{Limit: 3, PlanID: ""}) {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("returns an error on 401", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()

		c := New(srv.URL, "wrong")
		if _, err := c.SeatLimit(context.Background(), "org1"); err == nil {
			t.Fatal("expected an error on 401, got nil")
		}
	})

	t.Run("returns an error on 500", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		c := New(srv.URL, "secret")
		if _, err := c.SeatLimit(context.Background(), "org1"); err == nil {
			t.Fatal("expected an error on 500, got nil")
		}
	})

	t.Run("returns an error on timeout", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond)
			_, _ = w.Write([]byte(`{"seatLimit":3,"planId":""}`))
		}))
		defer srv.Close()

		c := New(srv.URL, "secret")
		c.http.Timeout = 10 * time.Millisecond
		if _, err := c.SeatLimit(context.Background(), "org1"); err == nil {
			t.Fatal("expected an error on timeout, got nil")
		}
	})
}
