package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

type Route struct {
	Headers  map[string]string `json:"headers,omitempty"`
	Method   string            `json:"method"`
	Status   int               `json:"status"`
	Response json.RawMessage   `json:"response,omitempty"`
}
type PathGroup struct {
	Handlers []Route     `json:"handlers,omitempty"`
	Children *jsonServer `json:"children,omitempty"`
}
type jsonServer map[string]PathGroup

var lh = slog.NewTextHandler(os.Stdout, nil).WithAttrs([]slog.Attr{slog.String("app", "json-server")})
var logger = slog.New(lh)

func main() {

	// we read the json file and use it to create our mux.
	serverFile := flag.String("file", "server.json", "the json file to use as the server")

	flag.Parse()

	var server jsonServer
	var err error

	if server, err = decodeServer(*serverFile); err != nil {
		log.Panicf("error decoding the server: %v", err)
	}

	mux := http.NewServeMux()

	if err = registerRoutes("", mux, &server); err != nil {
		log.Panicf("error registering the routes: %v", err)
	}
	server = nil
	if err = http.ListenAndServe(":3000", mux); err != nil {
		log.Panicf("error starting the server: %v", err)
	}

}

func registerRoutes(base string, mux *http.ServeMux, server *jsonServer) error {
	if server == nil {
		return nil
	}
	logger.Debug(
		"registering routes", "base", base,
	)
	for path, pathGroup := range *server {
		methods := make(map[string]func(http.ResponseWriter, *http.Request))
		// we want to make sure that unless it is the root of the server, the path does not end with a /
		if len(path) > 1 && path[len(path)-1] == '/' {
			return fmt.Errorf("path %s ends with a /", path)
		}
		// if base is "/" then we don't want to add it to the path because this signals the root of the server or root of the group
		if len(base) < 2 {
			logger.Warn("base route part of children, removing /. place the main route in the handlers")
			base = strings.TrimPrefix(base, "/")
		}
		path = base + path
		if pathGroup.Children != nil {
			if err := registerRoutes(base+path, mux, pathGroup.Children); err != nil {
				return err
			}
		}
		for _, route := range pathGroup.Handlers {
			// 1. Register the children

			// 2. Register the handlers
			methods[route.Method] = func(w http.ResponseWriter, r *http.Request) {
				// 1. Set the headers
				for k, v := range route.Headers {
					w.Header().Set(k, v)
				}
				// 2. Set the status code
				w.WriteHeader(route.Status)
				// 3. Write the response
				w.Write([]byte(route.Response))
				return
			}

		}
		logger.Info("[registering]", "path", path, "method", methods)
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			// we want strict routing.
			if r.URL.Path != path {
				http.NotFound(w, r)
				return
			}

			logger.Info("[request]", "path", r.URL.Path, "method", r.Method, "headers", r.Header)
			if method, ok := methods[r.Method]; ok {
				method(w, r)
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		})
	}
	return nil
}

func unmarshalServer(f io.Reader) (jsonServer, error) {

	var server jsonServer

	if err := json.NewDecoder(f).Decode(&server); err != nil {
		return nil, fmt.Errorf("error decoding the json file: %w", err)
	}
	return server, nil
}

func decodeServer(path string) (jsonServer, error) {
	file, err := os.Open(path)
	defer file.Close()

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("file does not exist: %w", err)
		}
		return nil, err
	}
	return unmarshalServer(file)
}
