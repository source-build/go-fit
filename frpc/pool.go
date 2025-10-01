package frpc

import (
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// ClientPool manages gRPC client connections across multiple services with intelligent
// connection pooling, load balancing, and automatic cleanup. It implements a two-tier
// architecture where each service has its own connection pool (ServiceConnectionPool)
// that can maintain multiple connections based on load.
//
// Architecture:
//   - Global ClientPool manages multiple services
//   - Each service has a ServiceConnectionPool with multiple connections
//   - Connections are automatically created/destroyed based on load
//   - Background cleanup removes idle and unhealthy connections
//
// Thread Safety:
// All operations are thread-safe and designed for high-concurrency scenarios.
// The pool uses read-write locks to minimize contention and atomic operations
// for connection usage tracking.
//
// Performance Characteristics:
//   - O(1) service pool lookup
//   - O(n) connection selection within service (where n = connections per service)
//   - Minimal lock contention through careful lock granularity
//   - Automatic load balancing using least-connection algorithm
type ClientPool struct {
	mu           sync.RWMutex                      // Protects servicePools map from concurrent access
	servicePools map[string]*ServiceConnectionPool // Maps service ID to its connection pool
	maxIdle      time.Duration                     // Maximum idle time before connection cleanup
	cleanupTick  time.Duration                     // Interval between cleanup cycles
	stopCh       chan struct{}                     // Signal channel for graceful shutdown
	once         sync.Once                         // Ensures Close() is called only once
	config       MultiConnConfig                   // Global configuration for all service pools
}

// MultiConnConfig defines the connection pooling behavior and limits.
// These settings control when new connections are created, how many
// connections are maintained, and the load balancing thresholds.
//
// Tuning Guidelines:
//   - ConcurrencyThreshold: Set based on expected request latency and throughput
//   - MaxConnectionsPerID: Balance between resource usage and performance
//   - MinConnectionsPerID: Ensure minimum availability for each service
type MultiConnConfig struct {
	// ConcurrencyThreshold is the maximum number of concurrent requests per connection
	// before a new connection is created. Higher values mean more requests per connection
	// but may increase latency under load. Typical values: 100-1000.
	ConcurrencyThreshold int

	// MaxConnectionsPerID limits the maximum number of connections per service.
	// This prevents resource exhaustion while allowing horizontal scaling.
	// Typical values: 5-20 depending on service characteristics.
	MaxConnectionsPerID int

	// MinConnectionsPerID ensures a minimum number of connections are maintained
	// per service to provide baseline availability. Usually set to 1-2.
	MinConnectionsPerID int
}

// PooledConnection wraps a gRPC connection with metadata for pool management.
// It tracks usage statistics and provides thread-safe access to connection state.
//
// Usage Tracking:
// The inUse field is managed atomically to track concurrent request count.
// This enables the load balancer to make intelligent routing decisions.
//
// Lifecycle Management:
// lastUsed timestamp enables idle connection cleanup, while the mutex
// protects against concurrent updates during connection state changes.
type PooledConnection struct {
	conn     *grpc.ClientConn // The underlying gRPC connection
	lastUsed time.Time        // Timestamp of last usage for idle cleanup
	inUse    int64            // Atomic counter of concurrent requests using this connection
	mu       sync.RWMutex     // Protects lastUsed field from concurrent updates
}

// Global connection pool instance using singleton pattern.
// This ensures consistent connection management across the entire application
// and prevents resource fragmentation from multiple pool instances.
var (
	globalPool     *ClientPool // Singleton instance of the connection pool
	globalPoolOnce sync.Once   // Ensures initialization happens exactly once
)

// PoolConfig contains configuration parameters for connection pool behavior.
// These settings control the cleanup and maintenance cycles that keep
// the pool healthy and prevent resource leaks.
//
// Configuration Guidelines:
//   - MaxIdleTime: Balance between connection reuse and resource cleanup
//   - CleanupTicker: More frequent cleanup uses more CPU but frees resources faster
type PoolConfig struct {
	// MaxIdleTime is the maximum duration a connection can remain idle
	// before being eligible for cleanup. Longer times improve connection
	// reuse but may hold resources longer. Default: 30 minutes.
	MaxIdleTime time.Duration

	// CleanupTicker is the interval between cleanup cycles that remove
	// idle and unhealthy connections. More frequent cleanup uses more CPU
	// but frees resources faster. Default: 5 minutes.
	CleanupTicker time.Duration

	// ConcurrencyThreshold is the maximum number of concurrent requests per connection
	// before a new connection is created. Higher values mean more requests per connection
	// but may increase latency under load. Typical values: 100-1000.
	ConcurrencyThreshold int

	// MaxConnectionsPerID limits the maximum number of connections per service.
	// This prevents resource exhaustion while allowing horizontal scaling.
	// Typical values: 5-20 depending on service characteristics.
	MaxConnectionsPerID int

	// MinConnectionsPerID ensures a minimum number of connections are maintained
	// per service to provide baseline availability. Usually set to 1-2.
	MinConnectionsPerID int
}

// InitPool initializes the global connection pool with the specified configuration.
// This function must be called before using any connection pool functionality.
// It uses the singleton pattern to ensure only one pool instance exists.
//
// The function is thread-safe and can be called multiple times - subsequent
// calls will be ignored. The background cleanup goroutine is started automatically.
//
// Default Configuration:
//   - MaxIdleTime: 30 seconds (for testing, normally 30 minutes)
//   - CleanupTicker: 20 seconds (for testing, normally 5 minutes)
//   - ConcurrencyThreshold: 1000 requests per connection
//   - MaxConnectionsPerID: 10 connections per service
//   - MinConnectionsPerID: 1 connection per service
//
// Parameters:
//   - config: Pool configuration parameters
func InitPool(config PoolConfig) {
	globalPoolOnce.Do(func() {
		// Apply default values for unspecified configuration
		if config.MaxIdleTime <= 0 {
			config.MaxIdleTime = 30 * time.Minute
		}
		if config.CleanupTicker <= 0 {
			config.CleanupTicker = 5 * time.Minute
		}

		// Configure multi-connection behavior with production-ready defaults
		multiConnConfig := MultiConnConfig{
			ConcurrencyThreshold: config.ConcurrencyThreshold,
			MaxConnectionsPerID:  config.MaxConnectionsPerID,
			MinConnectionsPerID:  config.MinConnectionsPerID,
		}

		// Apply configuration values
		if config.ConcurrencyThreshold < 10 {
			multiConnConfig.ConcurrencyThreshold = 500
		}
		if config.MaxConnectionsPerID < 5 {
			multiConnConfig.MaxConnectionsPerID = 5
		}
		if config.MinConnectionsPerID < 1 {
			multiConnConfig.MinConnectionsPerID = 1
		}

		// Initialize the global pool instance
		globalPool = &ClientPool{
			servicePools: make(map[string]*ServiceConnectionPool),
			maxIdle:      config.MaxIdleTime,
			cleanupTick:  config.CleanupTicker,
			stopCh:       make(chan struct{}),
			config:       multiConnConfig,
		}

		// Start background cleanup goroutine for automatic maintenance
		go globalPool.cleanup()
	})
}

// GetOrCreatePoolConnection retrieves or creates a pooled connection for the specified service.
// This is the main entry point for obtaining connections from the pool.
//
// The function implements a two-level lookup:
//  1. Find or create the service-specific connection pool
//  2. Get or create a connection within that service pool
//
// Load Balancing:
// The returned connection is selected using a least-connection algorithm,
// ensuring optimal load distribution across available connections.
//
// Connection Wrapping:
// The returned PooledConn wraps the underlying gRPC connection and provides
// automatic reference counting and cleanup when Close() is called.
//
// Parameters:
//   - id: Unique service identifier
//   - createFunc: Function to create new gRPC connections when needed
//
// Returns:
//   - *PooledConn: Wrapped connection with automatic cleanup
//   - error: Any error encountered during connection creation
//
// Usage Example:
//
//	conn, err := GetOrCreatePoolConnection("user-service", func() (*grpc.ClientConn, error) {
//		return grpc.NewClient("user-service:8080", grpc.WithInsecure())
//	})
//	if err != nil {
//		return err
//	}
//	defer conn.Close() // Important: always close to return to pool
func GetOrCreatePoolConnection(id string, createFunc func() (*grpc.ClientConn, error)) (*PooledConn, error) {
	if globalPool == nil {
		InitPool(PoolConfig{})
	}

	// Get or create service-specific connection pool
	sPool := globalPool.getOrCreateConnection(id)

	// Get or create connection within the service pool
	c, err := sPool.getOrCreateConnection(createFunc)
	if err != nil {
		return nil, err
	}

	// Wrap connection with pool management functionality
	return NewWrapConnection(id, c, sPool), nil
}

// getOrCreateConnection implements the service pool lookup with double-checked locking
// pattern for optimal performance. This method handles the creation of new service
// pools when they don't exist.
//
// Performance Optimization:
// Uses read lock for the common case (service pool exists) and only acquires
// write lock when creating new service pools. This minimizes lock contention
// in high-concurrency scenarios.
//
// Parameters:
//   - id: Service identifier
//
// Returns:
//   - *ServiceConnectionPool: The service-specific connection pool
func (p *ClientPool) getOrCreateConnection(id string) *ServiceConnectionPool {
	// Fast path: check if service pool exists with read lock
	p.mu.RLock()
	if servicePool, exists := p.servicePools[id]; exists {
		p.mu.RUnlock()
		return servicePool
	}
	p.mu.RUnlock()

	// Slow path: create new service pool with write lock
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check: another goroutine might have created the pool
	if servicePool, exists := p.servicePools[id]; exists {
		return servicePool
	}

	// Create new service connection pool with default configuration
	servicePool := &ServiceConnectionPool{
		serviceId:      id,
		connections:    make([]*PooledConnection, 0),
		config:         &p.config,
		lastActiveTime: time.Now(),
	}
	p.servicePools[id] = servicePool

	return servicePool
}

// isConnectionHealthy checks if a gRPC connection is in a usable state.
// This function is used throughout the pool to determine which connections
// can handle new requests.
//
// Healthy States:
//   - Ready: Connection is established and ready for requests
//   - Idle: Connection is available but not actively used
//   - Connecting: Connection is being established (considered healthy to allow time)
//
// Unhealthy States:
//   - TransientFailure: Connection failed and is retrying
//   - Shutdown: Connection is closed
//
// Parameters:
//   - conn: gRPC connection to check
//
// Returns:
//   - bool: true if connection can handle requests, false otherwise
func isConnectionHealthy(conn *grpc.ClientConn) bool {
	state := conn.GetState()
	return state == connectivity.Ready ||
		state == connectivity.Idle ||
		state == connectivity.Connecting
}

// isConnectionHealthy is a method version of the global function for backward compatibility.
// This method provides the same functionality as the global isConnectionHealthy function.
//
// Deprecated: Use the global isConnectionHealthy function instead.
func (p *ClientPool) isConnectionHealthy(conn *grpc.ClientConn) bool {
	state := conn.GetState()
	return state == connectivity.Ready ||
		state == connectivity.Idle ||
		state == connectivity.Connecting
}

// cleanup runs the background maintenance goroutine that periodically removes
// idle and unhealthy connections. This prevents resource leaks and maintains
// pool health over time.
//
// Cleanup Process:
//  1. Remove idle connections that exceed MaxIdleTime
//  2. Remove unhealthy connections that can't handle requests
//  3. Remove empty service pools that haven't been used
//  4. Respect minimum connection limits per service
//
// The cleanup runs on a timer and can be stopped gracefully via the stopCh channel.
func (p *ClientPool) cleanup() {
	ticker := time.NewTicker(p.cleanupTick)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanupExpiredConnections()
		case <-p.stopCh:
			return
		}
	}
}

// cleanupExpiredConnections performs the actual cleanup of idle and unhealthy connections.
// This method coordinates cleanup across all service pools and removes empty service pools.
//
// Service Pool Cleanup:
// Empty service pools are removed if they haven't been active for twice the MaxIdleTime.
// This prevents accumulation of unused service pools over time.
//
// Thread Safety:
// Uses appropriate locking to ensure cleanup doesn't interfere with active operations.
func (p *ClientPool) cleanupExpiredConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	var servicesToRemove []string

	// Clean up connections in each service pool
	for serviceId, servicePool := range p.servicePools {
		servicePool.cleanupExpiredConnections(p.maxIdle)

		// Check if service pool should be removed
		servicePool.mu.RLock()
		isEmpty := len(servicePool.connections) == 0
		servicePool.mu.RUnlock()

		if isEmpty {
			// Get last active time for empty service pool
			servicePool.lastActiveMu.RLock()
			lastActiveTime := servicePool.lastActiveTime
			servicePool.lastActiveMu.RUnlock()

			// Remove service pool if idle for twice the connection idle time
			servicePoolIdleThreshold := p.maxIdle * 2
			if now.Sub(lastActiveTime) > servicePoolIdleThreshold {
				servicesToRemove = append(servicesToRemove, serviceId)
			}
		}
	}

	// Remove idle service pools
	for _, serviceId := range servicesToRemove {
		if servicePool, exists := p.servicePools[serviceId]; exists {
			servicePool.Close()
			delete(p.servicePools, serviceId)
		}
	}
}

// Close gracefully shuts down the connection pool, closing all connections
// and stopping background goroutines. This method should be called during
// application shutdown to ensure proper resource cleanup.
//
// The method uses sync.Once to ensure it can be called multiple times safely.
// All connections are closed and resources are freed.
//
// Shutdown Process:
//  1. Stop the background cleanup goroutine
//  2. Close all service pools and their connections
//  3. Clear all internal data structures
func (p *ClientPool) Close() {
	p.once.Do(func() {
		// Signal cleanup goroutine to stop
		close(p.stopCh)

		// Close all service pools and connections
		p.mu.Lock()
		defer p.mu.Unlock()

		for _, servicePool := range p.servicePools {
			servicePool.Close()
		}
		p.servicePools = make(map[string]*ServiceConnectionPool)
	})
}

// ClosePool provides a convenient way to close the global connection pool.
// This function should be called during application shutdown to ensure
// all connections are properly closed and resources are freed.
//
// Usage Example:
//
//	func main() {
//		defer frpc.ClosePool() // Ensure cleanup on exit
//
//		// Application code using connection pool
//	}
func ClosePool() {
	if globalPool != nil {
		globalPool.Close()
	}
}
