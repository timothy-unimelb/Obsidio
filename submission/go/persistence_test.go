package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func postPrice(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	route(response, httptest.NewRequest(http.MethodPost, "/price", strings.NewReader(body)))
	return response
}

func TestPriceUpdateIsDurableAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prices.log")
	original := markets["AAPL"].priceBody()
	defer markets["AAPL"].setPrice(187.42)

	priceWAL = &priceLog{path: path}
	priceWAL.open()
	if response := postPrice(t, `{"symbol":"AAPL","price":190.5}`); response.Code != http.StatusOK {
		t.Fatalf("update failed: %d %s", response.Code, response.Body.String())
	}
	if got := request(t, "/price?symbol=AAPL").Body.String(); got != `{"symbol":"AAPL","price":190.5}` {
		t.Fatalf("GET after POST = %s", got)
	}
	priceWAL.close()

	// Simulate the kill: drop memory, restart from the log only.
	markets["AAPL"].setPrice(187.42)
	if markets["AAPL"].priceBody() != original {
		t.Fatal("reset failed")
	}
	priceWAL = &priceLog{path: path}
	priceWAL.open()
	defer priceWAL.close()
	if got := request(t, "/price?symbol=AAPL").Body.String(); got != `{"symbol":"AAPL","price":190.5}` {
		t.Fatalf("GET after replay = %s", got)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "AAPL 190.5\n" {
		t.Fatalf("unexpected log contents %q", data)
	}
}

func TestPriceUpdateValidation(t *testing.T) {
	priceWAL = &priceLog{path: filepath.Join(t.TempDir(), "p.log")}
	priceWAL.open()
	defer priceWAL.close()
	cases := map[string]int{
		`{"symbol":"NOPE","price":1}`:    http.StatusNotFound,
		`{"symbol":"AAPL","price":-1}`:   http.StatusBadRequest,
		`{"symbol":"AAPL"`:               http.StatusBadRequest,
		`{"symbol":"AAPL","price":1.25}`: http.StatusOK,
	}
	for body, want := range cases {
		if got := postPrice(t, body).Code; got != want {
			t.Fatalf("%s: got %d want %d", body, got, want)
		}
	}
	markets["AAPL"].setPrice(187.42)
	if got := request(t, "/stats?symbol=AAPL").Code; got != http.StatusOK {
		t.Fatalf("stats unaffected check failed: %d", got)
	}
}
