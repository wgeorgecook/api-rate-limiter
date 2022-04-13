package main

import (
	"sync"
	"time"
)

const (
	// ErrTooManyRequests is the error value thrown when our next
	// request would go above the available count.
	ErrTooManyRequests = rateLimitError("Too Many Requests")
	ErrClientMapExists = rateLimitError("Client rate limite map already instantiated")
)

// clientRateLimiterMap is a singleton of all clients we are holding
// in memory and their associated Limiter
var clientRateLimiterMap map[string]RateLimiter

// rateLimitError is a type we can use to build constant errors
// related to the rate limiter for stricter error checking.
type rateLimitError string

// Error inplements the error interface.
func (r rateLimitError) Error() string {
	return string(r)
}

// RateLimiter is an abstract type that defines the behavior we expect
// from a concrete rate limiter
type RateLimiter interface {
	GetRequestLimit() int
	GetRequestsAvailable() int
	GetTimeframeInterval() time.Duration
	IncrementRequestsUsed() error
	Clear()
	Shutdown()
}

// Limiter is a concrete implementation on the RateLimiter interface.
// It stores state pertaining to the requests allowed and how many
// requests that are currently counted against that limit across the
// given timeframe interval. This implementation uses a fixed window
// algorithm for simplicity.
type Limiter struct {
	sync.Mutex                      // prevents cuncurrent access
	usedRequests      int           // requests permitted during this timeframe
	allowedRequests   int           // requests allowed during this timeframe
	timeframeInterval time.Duration // interval to clear usedRequests
	doneChan          chan struct{} // channel for shutdown signal
}

// InitClientRateLimiterMap creates an instance of the
// clientRateLimiterMap
func InitClientRateLimiterMap() error {
	if clientRateLimiterMap != nil {
		return ErrClientMapExists
	}
	clientRateLimiterMap = make(map[string]RateLimiter)
	return nil
}

// GetRequestsAvailable returns the count of requests that are still
// allowed under the current time window.
func (l *Limiter) GetRequestsAvailable() int {
	return l.allowedRequests - l.usedRequests
}

// GetRequestLimit returns the maximum amount of requests allowed to
// process during an open window
func (l *Limiter) GetRequestLimit() int {
	return l.allowedRequests
}

// GetTimeframe returns the timeframe agreed upon for the fixed window
// rollover of request quotas
func (l *Limiter) GetTimeframeInterval() time.Duration {
	return l.timeframeInterval
}

// IncrementRequestsUsed adds another request to the total used
// requests.
func (l *Limiter) IncrementRequestsUsed() error {
	if l.usedRequests == l.allowedRequests {
		return ErrTooManyRequests
	}
	l.Lock()
	l.usedRequests++
	l.Unlock()
	return nil
}

// Clear resets the state of used requests to zero.
func (l *Limiter) Clear() {
	l.Lock()
	l.usedRequests = 0
	l.Unlock()
}

// Shutdown ends the ticker that keeps track of the window
func (l *Limiter) Shutdown() {
	l.doneChan <- struct{}{}
}

// startWindow is a blocking function that will call Clear() on the
// Limiter every tick of a ticker created with the Limiter's
// timeframeInterval. Will return when a signal is recieved on the
// Limiter's doneChan.
func (l *Limiter) startWindow() {
	ticker := time.NewTicker(l.timeframeInterval)
	for {
		select {
		case <-ticker.C:
			l.Clear()
		case <-l.doneChan:
			ticker.Stop()
			return
		}
	}

}

// NewLimter returns an instance of a Limiter with an allowed limit
// across a duration (in milliseconds) agreed upon with the client
// using the limiter.
func NewLimiter(allowedRequests, timeframeMilliseconds int) *Limiter {
	l := Limiter{
		usedRequests:    0,
		allowedRequests: allowedRequests,
		timeframeInterval: time.Duration(timeframeMilliseconds) *
			time.Millisecond,
		doneChan: make(chan struct{}),
	}

	go l.startWindow()

	return &l
}
