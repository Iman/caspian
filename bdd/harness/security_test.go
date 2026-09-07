// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestControlErrorsRemainJSONThroughResponseWrapper(t *testing.T) {
	payload := "</script><svg/onload=alert(1)>"
	body, err := json.Marshal(map[string]string{"defect": payload})
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{}
	for _, wrapped := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "wrapped"}[wrapped], func(t *testing.T) {
			var handler http.Handler = h
			if wrapped {
				handler = faulty{inner: h, d: defect{statusJSONFieldLost: true}}
			}
			r := httptest.NewRequest(http.MethodPost, "/__control/reset", bytes.NewReader(body))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status %d", w.Code)
			}
			if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
				t.Fatal("control response is not JSON")
			}
			if strings.Contains(w.Body.String(), "<") {
				t.Fatal("HTML in the error was not escaped")
			}
			var decoded map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(decoded["error"], payload) {
				t.Fatal("test did not exercise the reflected value")
			}
		})
	}
}
