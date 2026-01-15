package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

var (
	// Prometheus metrics
	cacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"endpoint"},
	)
	cacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"endpoint"},
	)
	dbCalls = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_calls_total",
			Help: "Total number of DB calls (simulated backend calls)",
		},
		[]string{"endpoint"},
	)
	singleflightShared = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "singleflight_shared_total",
			Help: "Total number of requests that shared a singleflight result",
		},
	)
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)
	inflightRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "inflight_requests",
			Help: "Number of requests currently being processed",
		},
		[]string{"endpoint"},
	)
)

func init() {
	prometheus.MustRegister(cacheHits)
	prometheus.MustRegister(cacheMisses)
	prometheus.MustRegister(dbCalls)
	prometheus.MustRegister(singleflightShared)
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(inflightRequests)
}

var (
	rdb          *redis.Client
	sfGroup      singleflight.Group
	cacheKey     = "product:popular"
	cacheTTL     = 5 * time.Second
	dbLatency    = 200 * time.Millisecond // Simulated DB latency
	requestCount int64
)

type Product struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Price     int    `json:"price"`
	FetchedAt string `json:"fetched_at"`
	RequestID int64  `json:"request_id"`
}

// simulateDBCall simulates a slow database call
func simulateDBCall(endpoint string) (*Product, error) {
	dbCalls.WithLabelValues(endpoint).Inc()
	time.Sleep(dbLatency) // Simulate DB latency
	reqID := atomic.AddInt64(&requestCount, 1)
	return &Product{
		ID:        1,
		Name:      "Popular Product",
		Price:     9800,
		FetchedAt: time.Now().Format(time.RFC3339Nano),
		RequestID: reqID,
	}, nil
}

// getFromCache tries to get data from Redis cache
func getFromCache(ctx context.Context) (*Product, bool) {
	val, err := rdb.Get(ctx, cacheKey).Result()
	if err == redis.Nil {
		return nil, false
	}
	if err != nil {
		log.Printf("Redis error: %v", err)
		return nil, false
	}
	var product Product
	if err := json.Unmarshal([]byte(val), &product); err != nil {
		log.Printf("Unmarshal error: %v", err)
		return nil, false
	}
	return &product, true
}

// setCache stores data in Redis cache
func setCache(ctx context.Context, product *Product) {
	data, err := json.Marshal(product)
	if err != nil {
		log.Printf("Marshal error: %v", err)
		return
	}
	if err := rdb.Set(ctx, cacheKey, data, cacheTTL).Err(); err != nil {
		log.Printf("Redis set error: %v", err)
	}
}

// withoutSingleflightHandler handles requests without singleflight
// This demonstrates the Cache Stampede problem
func withoutSingleflightHandler(w http.ResponseWriter, r *http.Request) {
	endpoint := "without_singleflight"
	start := time.Now()
	inflightRequests.WithLabelValues(endpoint).Inc()
	defer func() {
		inflightRequests.WithLabelValues(endpoint).Dec()
		requestDuration.WithLabelValues(endpoint).Observe(time.Since(start).Seconds())
	}()

	ctx := r.Context()

	// Check cache
	if product, ok := getFromCache(ctx); ok {
		cacheHits.WithLabelValues(endpoint).Inc()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		json.NewEncoder(w).Encode(product)
		return
	}

	// Cache miss - call DB directly (no protection against stampede)
	cacheMisses.WithLabelValues(endpoint).Inc()
	product, err := simulateDBCall(endpoint)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Store in cache
	setCache(ctx, product)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	json.NewEncoder(w).Encode(product)
}

// withSingleflightHandler handles requests with singleflight
// This prevents Cache Stampede by coalescing concurrent requests
func withSingleflightHandler(w http.ResponseWriter, r *http.Request) {
	endpoint := "with_singleflight"
	start := time.Now()
	inflightRequests.WithLabelValues(endpoint).Inc()
	defer func() {
		inflightRequests.WithLabelValues(endpoint).Dec()
		requestDuration.WithLabelValues(endpoint).Observe(time.Since(start).Seconds())
	}()

	ctx := r.Context()

	// Check cache first
	if product, ok := getFromCache(ctx); ok {
		cacheHits.WithLabelValues(endpoint).Inc()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		json.NewEncoder(w).Encode(product)
		return
	}

	// Cache miss - use singleflight to prevent stampede
	cacheMisses.WithLabelValues(endpoint).Inc()

	// singleflight.Do ensures only one goroutine fetches from DB
	// Other concurrent requests wait and share the result
	result, err, shared := sfGroup.Do(cacheKey, func() (interface{}, error) {
		// Double-check cache (another request might have populated it)
		if product, ok := getFromCache(ctx); ok {
			return product, nil
		}
		product, err := simulateDBCall(endpoint)
		if err != nil {
			return nil, err
		}
		setCache(ctx, product)
		return product, nil
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if shared {
		singleflightShared.Inc()
	}

	product := result.(*Product)
	w.Header().Set("Content-Type", "application/json")
	if shared {
		w.Header().Set("X-Cache", "SINGLEFLIGHT-SHARED")
	} else {
		w.Header().Set("X-Cache", "MISS")
	}
	w.Header().Set("X-Singleflight-Shared", fmt.Sprintf("%v", shared))
	json.NewEncoder(w).Encode(product)
}

// clearCacheHandler clears the cache (for testing)
func clearCacheHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := rdb.Del(ctx, cacheKey).Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Cache cleared"))
}

// healthHandler returns health status
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	// Initialize Redis client
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	rdb = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Printf("Connected to Redis at %s", redisAddr)

	// Setup HTTP routes
	http.HandleFunc("/api/without-singleflight", withoutSingleflightHandler)
	http.HandleFunc("/api/with-singleflight", withSingleflightHandler)
	http.HandleFunc("/api/clear-cache", clearCacheHandler)
	http.HandleFunc("/health", healthHandler)
	http.Handle("/metrics", promhttp.Handler())

	port := getEnv("PORT", "8080")
	log.Printf("Server starting on port %s", port)
	log.Printf("Endpoints:")
	log.Printf("  - GET /api/without-singleflight (Cache Stampede prone)")
	log.Printf("  - GET /api/with-singleflight (Protected by singleflight)")
	log.Printf("  - GET /api/clear-cache (Clear Redis cache)")
	log.Printf("  - GET /metrics (Prometheus metrics)")
	log.Printf("  - GET /health (Health check)")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
