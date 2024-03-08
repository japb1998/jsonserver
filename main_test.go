package main

import (
	"bytes"
	"net/http"
	"testing"
)

func TestServer(t *testing.T) {
	input := bytes.NewBufferString(`{
		"/": {
			"handlers": [{
			 
				"method": "GET",
				"status": 200,
				"response": {
					"message": "Hello, world!"
				},
				"headers": {
					"Content-Type": "application/json"
				}
			}],
			"children": {
				"/newpath": {
					"handlers": [{
						"method": "GET",
						"status": 200,
						"headers": {
							"Content-Type": "application/json"
						}
					}],
					"children": {
						"/newpath2": {
							"handlers": [{
								"method": "POST",
								"status": 204,
								"headers": {
									"Content-Type": "application/json"
								}
							}]
						}
					}
				}
			}
		}
	}`)

	s, err := unmarshalServer(input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	mux := http.NewServeMux()

	if err := registerRoutes("", mux, &s); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	go func() {
		http.ListenAndServe(":3000", mux)
	}()

	for i := 0; i < 4; i++ {
		resp, err := http.Post("http://localhost:3000/newpath/newpath2", "text/plain", nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp.StatusCode != 204 {
			t.Errorf("expected status code 204, got %d", resp.StatusCode)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type to be application/json, got %s", resp.Header.Get("Content-Type"))
		}
	}
}
