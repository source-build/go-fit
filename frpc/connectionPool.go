package frpc

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// ServiceConnectionPool manages multiple connections for a single service.
// It implements intelligent load balancing by tracking connection usage
// and automatically scaling the number of connections based on demand.
//
// Load Balancing Strategy:
//  1. Find the connection with the lowest active request count
//  2. If load is below threshold, reuse existing connection
//  3. If all connections are overloaded and limit not reached, create new connection
//  4. If at connection limit, use least loaded connection
//
// Connection Lifecycle:
//   - Connections are created on-demand when load exceeds thresholds
//   - Idle connections are automatically cleaned up after maxIdle time
//   - Unhealthy connections are removed and replaced automatically
//   - Minimum connection count is maintained per service
type ServiceConnectionPool struct {
	serviceId      string              // Unique identifier for the service
	connections    []*PooledConnection // Array of pooled connections for this service
	mu             sync.RWMutex        // Protects connections array from concurrent access
	config         *MultiConnConfig    // Configuration reference from parent pool
	lastActiveTime time.Time           // Last time this service pool was accessed
	lastActiveMu   sync.RWMutex        // Protects lastActiveTime from concurrent updates
}

// getOrCreateConnection implements the core load balancing and connection management
// logic for a service connection pool. This method is the heart of the multi-connection
// pooling system, implementing intelligent connection selection and creation.
//
// Load Balancing Algorithm:
//  1. Scan all connections to find the one with lowest active request count
//  2. If the best connection has load below ConcurrencyThreshold, use it immediately
//  3. If all connections are overloaded but we haven't reached MaxConnectionsPerID, create new connection
//  4. If at connection limit, use the least loaded connection regardless of its load
//
// Performance Characteristics:
//   - O(n) where n is the number of connections for this service (typically 1-10)
//   - Atomic operations for thread-safe usage counting
//   - Minimal lock time through careful lock management
//
// Thread Safety:
// Uses read lock during connection scanning and upgrades to write lock only
// when creating new connections. This minimizes contention during normal operation.
//
// Parameters:
//   - createFunc: Function to create new gRPC connections when needed
//
// Returns:
//   - *PooledConnection: Selected or newly created connection
//   - error: Any error encountered during connection creation
func (scp *ServiceConnectionPool) getOrCreateConnection(createFunc func() (*grpc.ClientConn, error)) (*PooledConnection, error) {
	// Update service pool activity timestamp for cleanup tracking
	scp.updateLastActiveTime()

	scp.mu.RLock()

	// Phase 1: Find the connection with the lowest active request count
	var bestConn *PooledConnection
	var minActiveReqs int64 = math.MaxInt64

	for _, pooledConn := range scp.connections {
		if isConnectionHealthy(pooledConn.conn) {
			// Get current active request count atomically
			activeReqs := atomic.LoadInt64(&pooledConn.inUse)

			if activeReqs < minActiveReqs {
				minActiveReqs = activeReqs
				bestConn = pooledConn
			}
		}
	}

	// Phase 2: Use existing connection if load is acceptable
	if bestConn != nil {
		// If the best connection's load is below threshold, use it immediately
		if minActiveReqs < int64(scp.config.ConcurrencyThreshold) {
			// Atomically increment usage counter
			atomic.AddInt64(&bestConn.inUse, 1)

			// Update last used timestamp with proper locking
			bestConn.mu.Lock()
			bestConn.lastUsed = time.Now()
			bestConn.mu.Unlock()

			scp.mu.RUnlock()
			return bestConn, nil
		}
	}

	// Phase 3: Consider creating new connection if all existing ones are overloaded
	// Check if we can create a new connection (haven't reached the limit)
	if len(scp.connections) < scp.config.MaxConnectionsPerID {
		scp.mu.RUnlock()
		// Create new connection to handle the load
		return scp.createNewConnection(createFunc)
	}

	// Phase 4: Use least loaded connection even if it's overloaded
	// We've reached the connection limit, so use the best available connection
	if bestConn != nil {
		atomic.AddInt64(&bestConn.inUse, 1)
		bestConn.mu.Lock()
		bestConn.lastUsed = time.Now()
		bestConn.mu.Unlock()
		scp.mu.RUnlock()
		return bestConn, nil
	}

	scp.mu.RUnlock()
	return nil, fmt.Errorf("no healthy connections available")
}

// createNewConnection creates a new gRPC connection and adds it to the service pool.
// This method handles the thread-safe creation and initialization of new connections
// when the existing connections are overloaded.
//
// Connection Initialization:
//   - Creates the underlying gRPC connection using the provided factory function
//   - Wraps it in a PooledConnection with usage tracking
//   - Starts background monitoring for connection health
//   - Adds to the service pool's connection array
//
// Concurrency Control:
// Uses write lock to ensure thread-safe modification of the connections array.
// Includes double-checking of connection limits to handle race conditions.
//
// Monitoring:
// Automatically starts a goroutine to monitor the connection's health state
// and remove it from the pool if it becomes permanently unhealthy.
//
// Parameters:
//   - createFunc: Function to create the underlying gRPC connection
//
// Returns:
//   - *PooledConnection: Newly created and initialized connection
//   - error: Any error encountered during connection creation
func (scp *ServiceConnectionPool) createNewConnection(createFunc func() (*grpc.ClientConn, error)) (*PooledConnection, error) {
	scp.mu.Lock()
	defer scp.mu.Unlock()

	// Double-check connection limit in case another goroutine created a connection
	if len(scp.connections) >= scp.config.MaxConnectionsPerID {
		// Limit reached, fall back to selecting least used existing connection
		return scp.selectLeastUsedConnection()
	}

	// Create the underlying gRPC connection
	conn, err := createFunc()
	if err != nil {
		return nil, err
	}

	// Wrap in PooledConnection with initial usage count of 1
	pooledConn := &PooledConnection{
		conn:     conn,
		lastUsed: time.Now(),
		inUse:    1, // New connection is immediately in use
	}

	// Add to the service pool's connection array
	scp.connections = append(scp.connections, pooledConn)

	// Start background health monitoring for this connection
	go scp.watchConnectionState(conn)

	return pooledConn, nil
}

// selectLeastUsedConnection finds and returns the connection with the lowest
// active request count. This method is used when the connection limit has been
// reached and we need to select the best available connection.
//
// Selection Criteria:
//   - Only considers healthy connections
//   - Selects connection with minimum active request count
//   - Atomically increments usage counter of selected connection
//
// This method assumes the caller holds the appropriate lock.
//
// Returns:
//   - *PooledConnection: Connection with lowest load
//   - error: If no healthy connections are available
func (scp *ServiceConnectionPool) selectLeastUsedConnection() (*PooledConnection, error) {
	var bestConn *PooledConnection
	var minActiveReqs int64 = math.MaxInt64

	// Find connection with minimum active requests
	for _, pooledConn := range scp.connections {
		if isConnectionHealthy(pooledConn.conn) {
			activeReqs := atomic.LoadInt64(&pooledConn.inUse)
			if activeReqs < minActiveReqs {
				minActiveReqs = activeReqs
				bestConn = pooledConn
			}
		}
	}

	if bestConn != nil {
		// Increment usage counter and update timestamp
		atomic.AddInt64(&bestConn.inUse, 1)
		bestConn.mu.Lock()
		bestConn.lastUsed = time.Now()
		bestConn.mu.Unlock()
		return bestConn, nil
	}

	return nil, fmt.Errorf("no healthy connections available")
}

// watchConnectionState monitors a gRPC connection's state changes and automatically
// removes it from the pool if it becomes permanently unhealthy. This background
// monitoring ensures the pool maintains only usable connections.
//
// Monitored States:
//   - Shutdown: Connection is closed, remove immediately
//   - TransientFailure: Give one retry opportunity, then remove if still failing
//   - Other states: Continue monitoring
//
// Automatic Cleanup:
// When a connection becomes permanently unhealthy, it's automatically removed
// from the pool to prevent it from being selected for new requests.
//
// Retry Logic:
// For TransientFailure state, the monitor waits 30 seconds to allow for
// automatic reconnection before deciding to remove the connection.
//
// Parameters:
//   - conn: gRPC connection to monitor
func (scp *ServiceConnectionPool) watchConnectionState(conn *grpc.ClientConn) {
	ctx := context.Background()

	for {
		currentState := conn.GetState()

		// Wait for state change or context cancellation
		if !conn.WaitForStateChange(ctx, currentState) {
			// Context cancelled, exit monitoring
			return
		}

		newState := conn.GetState()

		// Handle permanent connection closure
		if newState == connectivity.Shutdown {
			scp.removeConnection(conn)
			return
		}

		// Handle transient failures with retry logic
		if newState == connectivity.TransientFailure {
			// Give the connection a chance to recover
			time.Sleep(30 * time.Second)

			// Check if it's still failing after retry period
			if conn.GetState() == connectivity.TransientFailure {
				scp.removeConnection(conn)
				return
			}
		}
	}
}

// removeConnection safely removes a connection from the service pool.
// This method handles the thread-safe removal of connections that have
// become permanently unhealthy or closed.
//
// Cleanup Process:
//  1. Acquire write lock to ensure exclusive access
//  2. Find the connection in the array
//  3. Close the underlying gRPC connection
//  4. Remove from the connections array
//
// Array Management:
// Uses slice operations to remove the connection while maintaining
// the integrity of the connections array.
//
// Parameters:
//   - conn: gRPC connection to remove from the pool
func (scp *ServiceConnectionPool) removeConnection(conn *grpc.ClientConn) {
	scp.mu.Lock()
	defer scp.mu.Unlock()

	// Find and remove the connection from the array
	for i, pooledConn := range scp.connections {
		if pooledConn.conn == conn {
			// Close the underlying connection
			_ = pooledConn.conn.Close()

			// Remove from slice by combining parts before and after the element
			scp.connections = append(scp.connections[:i], scp.connections[i+1:]...)
			break
		}
	}
}

// cleanupExpiredConnections removes idle and unhealthy connections from the service pool.
// This method is called periodically by the global cleanup process to maintain
// pool health and prevent resource leaks.
//
// Cleanup Criteria:
//  1. Connection must not be in use (inUse <= 0)
//  2. Connection must exceed the maximum idle time OR be unhealthy
//  3. Must maintain minimum connection count per service
//
// Cleanup Process:
//   - Scans all connections to identify candidates for removal
//   - Marks connections for removal without immediately modifying the array
//   - Removes marked connections in reverse order to avoid index issues
//
// Resource Management:
// Properly closes underlying gRPC connections before removing them
// from the pool to prevent resource leaks.
//
// Parameters:
//   - maxIdle: Maximum idle time before a connection is eligible for cleanup
func (scp *ServiceConnectionPool) cleanupExpiredConnections(maxIdle time.Duration) {
	scp.mu.Lock()
	defer scp.mu.Unlock()

	now := time.Now()
	var toRemove []int

	// Scan all connections for cleanup candidates
	for i, pooledConn := range scp.connections {
		// Get last used timestamp safely
		pooledConn.mu.RLock()
		lastUsed := pooledConn.lastUsed
		pooledConn.mu.RUnlock()

		// Check current usage count
		inUseCount := atomic.LoadInt64(&pooledConn.inUse)

		// Determine if connection should be cleaned up
		shouldCleanup := inUseCount <= 0 &&
			len(scp.connections) > scp.config.MinConnectionsPerID &&
			(now.Sub(lastUsed) > maxIdle || !isConnectionHealthy(pooledConn.conn))

		if shouldCleanup {
			// Close the underlying connection
			_ = pooledConn.conn.Close()

			// Mark for removal (will be removed in reverse order)
			toRemove = append(toRemove, i)
		}
	}

	// Remove marked connections in reverse order to avoid index shifting issues
	for i := len(toRemove) - 1; i >= 0; i-- {
		idx := toRemove[i]
		scp.connections = append(scp.connections[:idx], scp.connections[idx+1:]...)
	}
}

// updateLastActiveTime updates the service pool's last activity timestamp.
// This timestamp is used by the global cleanup process to determine when
// to remove empty service pools that haven't been used recently.
//
// Thread Safety:
// Uses a dedicated mutex to protect the lastActiveTime field from
// concurrent updates during high-frequency access patterns.
//
// Usage Tracking:
// Called whenever the service pool is accessed for connection operations,
// ensuring accurate tracking of service pool activity.
func (scp *ServiceConnectionPool) updateLastActiveTime() {
	scp.lastActiveMu.Lock()
	scp.lastActiveTime = time.Now()
	scp.lastActiveMu.Unlock()
}

// Close gracefully shuts down the service connection pool by closing all
// connections and clearing the connections array. This method is called
// during application shutdown or when removing idle service pools.
//
// Shutdown Process:
//  1. Acquire write lock for exclusive access
//  2. Close all underlying gRPC connections
//  3. Clear the connections array
//
// Resource Cleanup:
// Ensures all gRPC connections are properly closed to prevent resource
// leaks and connection accumulation at the server side.
//
// Thread Safety:
// Uses write lock to ensure no other operations are accessing the
// connections array during shutdown.
func (scp *ServiceConnectionPool) Close() {
	scp.mu.Lock()
	defer scp.mu.Unlock()

	// Close all connections in the pool
	for _, pooledConn := range scp.connections {
		_ = pooledConn.conn.Close()
	}

	// Clear the connections array
	scp.connections = make([]*PooledConnection, 0)
}
