package main

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestJSONResponse(t *testing.T) {
    rr := httptest.NewRecorder()
    jsonResponse(rr, http.StatusNotImplemented, response{Service: "transaction-service", Status: "not-implemented"})
    if rr.Code != http.StatusNotImplemented || !strings.Contains(rr.Body.String(), `"not-implemented"`) {
        t.Fatalf("unexpected response: %d %s", rr.Code, rr.Body.String())
    }
}
