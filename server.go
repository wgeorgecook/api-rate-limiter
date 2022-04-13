package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

const (
	HttpTimeout       = 15 * time.Second                           // default timeout for http requests
	HttpServerAddress = ":8080"                                    // default address to listen for http requests
	HttpBadRequest    = "Bad Request"                              // http request was invalid
	HttpNotFound      = "Not Found"                                // resource requested was not found
	HttpInternalError = internalHttpError("Internal Server Error") // something internal went wrong
)

// internalHttpError is a type we can use to build constant errors
// related to the http handlers that can throw errors unrelated to
// the user's request
type internalHttpError string

// Error inplements the error interface.
func (i internalHttpError) Error() string {
	return string(i)
}

// AvailableRequestsResponse is the data we return to a user
// requesting their available requests
type AvailableRequestsResponse struct {
	Limit     int           `json:"limit"`     // total number of requests in a time frame
	Available int           `json:"available"` // available requests in the current time frame
	Timeframe time.Duration `json:"timeframe"` // fixed window timeframe
}

// HttpServerOptions are values that can be passed in to initialize an
// HTTP server.
type HttpServerOptions struct {
	ReadTimeout, WriteTimeout time.Duration
	Address                   string
}

// InitServer instantializes an HTTP server struct with optional
// overrides to default values.
func InitServer(options *HttpServerOptions) *http.Server {
	addr := HttpServerAddress
	readTimeout, writeTimeout := HttpTimeout, HttpTimeout
	if options != nil {
		if options.Address != "" {
			addr = options.Address
		}
		if options.ReadTimeout != 0 {
			readTimeout = options.ReadTimeout
		}
		if options.WriteTimeout != 0 {
			writeTimeout = options.WriteTimeout
		}
	}
	return &http.Server{
		Addr:         addr,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		Handler:      initRouter(),
	}
}

// initRouter initializes a new gorilla/mux router and registers the
// handler functions on it
func initRouter() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/{clientId}/requests-available",
		getAvailableRequests).Methods(http.MethodGet)
	return r
}

// StartServer is a blocking function that begins listening on the
// address provided when initializing the server.
func StartServer(s *http.Server) {
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		// TODO: this should be a captured error for investigation
		panic(err)
	}
}

// ShutdownServer blocks until all connections are either closed or
// timed out and then removes the server.
func ShutdownServer(s *http.Server, ctx context.Context) error {
	if err := s.Shutdown(ctx); err != nil {
		return err
	}

	return nil
}

// getAvailableRequests returns the available requests for the provided
// clientId.
func getAvailableRequests(w http.ResponseWriter, r *http.Request) {
	// pull out the clientId from the request
	vars := mux.Vars(r)
	clientId, ok := vars["clientId"]
	if !ok || clientId == "" {
		http.Error(w, HttpBadRequest, http.StatusBadRequest)
		return
	}

	// check if this clientId was instantiated in the rate limit to
	// client map
	clientRateLimiter, ok := clientRateLimiterMap[clientId]
	if !ok {
		// we haven't initialized a limiter with this client yet
		http.Error(w, HttpNotFound, http.StatusNotFound)
		return
	}

	// build return data the client
	returnData := AvailableRequestsResponse{
		Limit:     clientRateLimiter.GetRequestLimit(),
		Available: clientRateLimiter.GetRequestsAvailable(),
		Timeframe: clientRateLimiter.GetTimeframeInterval(),
	}

	// serialize it to json
	returnBytes, err := json.Marshal(&returnData)
	if err != nil {
		// something went wrong, and it wasn't the caller's fault
		http.Error(w, HttpInternalError.Error(), http.StatusInternalServerError)
		return
	}

	// write the json back on the response
	// TODO: this error should get logged somewhere for review!
	_, _ = w.Write(returnBytes)
	return
}
