package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSONResponse(t *testing.T) {
	rr := httptest.NewRecorder()
	jsonResponse(rr, http.StatusOK, response{Service: "game-server", Status: "ok"})
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"game-server"`) {
		t.Fatalf("unexpected response: %d %s", rr.Code, rr.Body.String())
	}
}
