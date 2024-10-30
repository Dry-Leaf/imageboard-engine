package main

import (
    "sync"

    "golang.org/x/time/rate"
)

// IPRateLimiter .
type IPRateLimiter struct {
    ips         map[string][]*rate.Limiter
    mu          sync.RWMutex
    refill      []rate.Limit
    bucket      []int
}

// NewIPRateLimiter .
func NewIPRateLimiter(r []rate.Limit, b []int) *IPRateLimiter {
    i := &IPRateLimiter{
        ips:        make(map[string][]*rate.Limiter),
        refill:     r,
        bucket:     b, 
    }

    return i
}

// AddIP creates a new rate limiter and adds it to the ips map,
// using the IP address as the key
func (i *IPRateLimiter) AddIP(ip string, sel int) *rate.Limiter {
    i.mu.Lock()
    defer i.mu.Unlock()

    var limiters []*rate.Limiter

    for idx, r := range(i.refill) {
        new_limiter := rate.NewLimiter(r, i.bucket[idx])
        limiters = append(limiters, new_limiter)
    } 
    
    i.ips[ip] = limiters
    return limiters[sel]
}

// GetLimiter returns the rate limiter for the provided IP address if it exists.
// Otherwise calls AddIP to add IP address to the map
func (i *IPRateLimiter) GetLimiter(ip string, sel int) *rate.Limiter {
    i.mu.Lock()
    limiters, exists := i.ips[ip]
    i.mu.Unlock()
    
    if !exists {
       return i.AddIP(ip, sel)
    }

    return limiters[sel]
}
