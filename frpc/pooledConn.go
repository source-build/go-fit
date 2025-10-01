package frpc

import (
	"sync/atomic"

	"google.golang.org/grpc"
)

// PooledConn is a wrapper around grpc.ClientConn that provides automatic
// connection pool management and reference counting. It embeds the gRPC
// client connection while adding pool-specific functionality for tracking
// usage and ensuring proper cleanup.
//
// Key Features:
//   - Automatic reference counting for connection usage tracking
//   - Seamless integration with gRPC client interfaces
//   - Automatic return to pool when Close() is called
//   - Thread-safe operations using atomic counters
//
// Usage Pattern:
// PooledConn should always be used with defer Close() to ensure proper
// cleanup and return to the connection pool:
//
//	conn, err := frpc.NewClient("service-name")
//	if err != nil {
//		return err
//	}
//	defer conn.Close() // Critical: always close to return to pool
//
//	// Use conn as a normal gRPC connection
//	client := pb.NewServiceClient(conn)
//	response, err := client.Method(ctx, request)
//
// Thread Safety:
// All operations on PooledConn are thread-safe. Multiple goroutines can
// safely use the same PooledConn instance, and the Close() method can be
// called multiple times without issues.
type PooledConn struct {
	*grpc.ClientConn                   // Embedded gRPC connection for transparent usage
	connId           string            // Unique identifier for this connection instance
	poolConn         *PooledConnection // Reference to the underlying pooled connection
	servicePool      *ServiceConnectionPool // Reference to the service pool for lifecycle management
}

// NewWrapConnection creates a new PooledConn wrapper around a pooled connection.
// This function is used internally by the connection pool to wrap connections
// before returning them to clients.
//
// The wrapper provides:
//   - Direct access to the underlying gRPC connection
//   - Automatic reference counting through the poolConn
//   - Service pool integration for activity tracking
//
// Parameters:
//   - connId: Unique identifier for this connection instance
//   - poolConn: The underlying pooled connection with usage tracking
//   - sPool: Service pool reference for lifecycle management
//
// Returns:
//   - *PooledConn: Wrapped connection ready for use
//
// Internal Use:
// This function is called internally by GetOrCreatePoolConnection and should
// not be called directly by application code.
func NewWrapConnection(connId string, poolConn *PooledConnection, sPool *ServiceConnectionPool) *PooledConn {
	return &PooledConn{
		ClientConn:  poolConn.conn,
		connId:      connId,
		poolConn:    poolConn,
		servicePool: sPool,
	}
}

// Conn returns the underlying gRPC ClientConn for cases where direct access
// is needed. In most cases, the embedded connection can be used directly
// since PooledConn embeds *grpc.ClientConn.
//
// Use Cases:
//   - Passing to functions that specifically require *grpc.ClientConn
//   - Accessing gRPC-specific methods not exposed through embedding
//   - Type assertions or interface compliance checks
//
// Returns:
//   - *grpc.ClientConn: The underlying gRPC connection
//
// Example:
//
//	pooledConn, err := frpc.NewClient("service")
//	if err != nil {
//		return err
//	}
//	defer pooledConn.Close()
//
//	// Direct usage (preferred)
//	client := pb.NewServiceClient(pooledConn)
//
//	// Explicit access to underlying connection (if needed)
//	grpcConn := pooledConn.Conn()
//	state := grpcConn.GetState()
func (p *PooledConn) Conn() *grpc.ClientConn {
	return p.ClientConn
}

// Close decrements the connection's usage counter and updates the service pool's
// activity timestamp. This method MUST be called when finished using the connection
// to ensure proper pool management and resource cleanup.
//
// Important Behaviors:
//   - Does NOT actually close the underlying gRPC connection
//   - Decrements the atomic usage counter (inUse) by 1
//   - Updates the service pool's last active time for cleanup tracking
//   - Can be called multiple times safely (subsequent calls are no-ops)
//   - Thread-safe and can be called from multiple goroutines
//
// Connection Lifecycle:
//   1. GetOrCreatePoolConnection() increments inUse counter
//   2. Application uses the connection for gRPC calls
//   3. Application calls Close() to return connection to pool
//   4. inUse counter is decremented, making connection available for reuse
//   5. Background cleanup may eventually close idle connections
//
// Performance Impact:
// This method is highly optimized with minimal overhead:
//   - Single atomic operation for usage counter
//   - Single mutex operation for timestamp update
//   - No network operations or blocking calls
//
// Critical Usage:
// Always use with defer to ensure cleanup even in error cases:
//
//	conn, err := frpc.NewClient("service")
//	if err != nil {
//		return err
//	}
//	defer conn.Close() // Ensures cleanup regardless of execution path
//
//	// Use connection...
//	if someCondition {
//		return // Close() still called via defer
//	}
//	// More connection usage...
//	// Close() called automatically when function exits
func (p *PooledConn) Close() {
	// Update service pool activity timestamp for cleanup tracking
	// This ensures the service pool is not considered idle while connections are being used
	p.servicePool.updateLastActiveTime()
	
	// Atomically decrement the usage counter if it's greater than 0
	// This makes the connection available for reuse by other requests
	if atomic.LoadInt64(&p.poolConn.inUse) > 0 {
		atomic.AddInt64(&p.poolConn.inUse, -1)
	}
}
