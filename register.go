// Package fit provides service registration and discovery functionality for microservices.
// It implements a robust service registry using etcd as the backend storage with
// automatic retry mechanisms, connection recovery, and memory optimizations.
package fit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// RegisterValue represents the service registration data stored in etcd.
// It contains essential service information including network details and metadata.
type RegisterValue struct {
	// Timestamp indicates when the service was registered (Unix timestamp)
	Timestamp int64 `json:"timestamp"`

	// IP is the service's IP address
	IP string `json:"ip"`

	// Port is the service's port number
	Port string `json:"port"`

	// Meta contains additional service metadata (e.g., version, weight, tags)
	Meta H `json:"meta"`
}

// Json serializes the RegisterValue to JSON string format.
// Uses bytes.Buffer for efficient memory allocation during JSON encoding.
func (r RegisterValue) Json() string {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	if err := encoder.Encode(r); err != nil {
		panic(err)
	}
	return buf.String()
}

// registerValuePool is an object pool that reduces memory allocation overhead
// by reusing RegisterValue instances across multiple registration operations.
// This optimization significantly reduces GC pressure in high-frequency scenarios.
var registerValuePool = sync.Pool{
	New: func() interface{} {
		return &RegisterValue{}
	},
}

// RegisterOptions contains all configuration parameters for service registration.
// It provides comprehensive control over service discovery behavior, retry policies,
// and connection management.
type RegisterOptions struct {
	// Namespace provides logical isolation for services across different environments.
	// Services in different namespaces cannot discover each other.
	// Default: "default"
	Namespace string

	// Key is the unique service identifier used for service discovery.
	// Clients use this key to locate and connect to service instances.
	// Required field - cannot be empty.
	Key string

	// ServiceType categorizes the service for organizational purposes.
	// Supported values: "api" (REST/HTTP services), "rpc" (gRPC services)
	// Default: "rpc"
	ServiceType string

	// IP is the service's network address. Special values:
	// - "*": Auto-detect outbound IP address
	// - "": Invalid, will cause panic
	// - Specific IP: Use provided address
	IP string

	// Port is the service's listening port number.
	// Required field - cannot be empty.
	Port string

	// EtcdConfig contains etcd client configuration parameters.
	// Must include at least one endpoint in the Endpoints slice.
	EtcdConfig clientv3.Config

	// Logger is an optional zap logger instance for structured logging.
	// If nil, the service will use standard fmt.Printf for logging.
	Logger *zap.Logger

	// MaxRetryAttempts controls the retry behavior when registration fails:
	// - 0: Unlimited retries (recommended for production)
	// - >0: Limited retries, service exits after reaching the limit
	// - <0: Reset to 0 (unlimited retries)
	MaxRetryAttempts int

	// TimeToLive specifies the lease duration in seconds for service registration.
	// The service must renew its lease within this interval to remain discoverable.
	// Default: 10 seconds
	TimeToLive int64

	// Meta contains custom service metadata (e.g., version, weight, health check URL).
	// This information is available to service discovery clients.
	Meta H
}

// RegisterService manages the complete lifecycle of service registration.
// It handles initial registration, lease renewal, connection recovery,
// and graceful shutdown with comprehensive error handling.
type RegisterService struct {
	// opt stores the registration configuration
	opt RegisterOptions

	// fullKey is the complete etcd key path for this service instance
	// Format: "<Namespace>/services/<ServiceType>/<Key>/<LeaseId>"
	fullKey string

	// client is the etcd client instance for all operations
	client *clientv3.Client

	// leaseId is the current etcd lease identifier
	leaseId clientv3.LeaseID

	// keepAliveCh receives lease renewal responses from etcd
	keepAliveCh <-chan *clientv3.LeaseKeepAliveResponse

	// quitCh signals when the service should terminate due to registration failure
	quitCh chan struct{}

	// closeCh signals when the service is being gracefully shut down
	closeCh chan struct{}
}

// NewRegisterService creates and initializes a new service registration instance.
// It validates configuration parameters, establishes etcd connection,
// and starts the registration process immediately.
//
// Parameters:
//   - opt: Configuration options for service registration
//
// Returns:
//   - *RegisterService: Initialized service registration instance
//   - error: Configuration validation or registration errors
//
// Example:
//
//	reg, err := fit.NewRegisterService(fit.RegisterOptions{
//	    Namespace:   "production",
//	    Key:         "user",
//	    ServiceType: "rpc",
//	    IP:          "*", // Auto-detect IP
//	    Port:        "8080",
//	    EtcdConfig: clientv3.Config{
//	        Endpoints: []string{"localhost:2379"},
//	    },
//	    MaxRetryAttempts: 0, // Unlimited retries
//	    TimeToLive:       10,
//	})
func NewRegisterService(opt RegisterOptions) (*RegisterService, error) {
	// Apply default values for optional parameters
	if opt.Namespace == "" {
		opt.Namespace = "default"
	}

	// Validate required parameters
	if opt.Key == "" {
		panic("service name (Key) cannot be empty")
	}

	if opt.Port == "" {
		panic("Port cannot be empty")
	}

	// Handle IP address resolution
	if opt.IP == "*" {
		var err error
		opt.IP, err = GetOutBoundIP()
		if err != nil {
			opt.IP = "127.0.0.1" // Fallback to localhost
		}
	} else if opt.IP == "" {
		panic("IP cannot be empty")
	}

	// Validate etcd configuration
	if len(opt.EtcdConfig.Endpoints) == 0 {
		panic("The connection endpoint is empty")
	}

	// Normalize service type
	if opt.ServiceType != "api" && opt.ServiceType != "rpc" {
		opt.ServiceType = "rpc"
	}
	opt.ServiceType = strings.ToLower(opt.ServiceType)

	// Apply default lease duration
	if opt.TimeToLive < 1 {
		opt.TimeToLive = 10
	}

	// Configure retry behavior
	// MaxRetryAttempts = 0 means unlimited retries (recommended for production)
	// MaxRetryAttempts < 0 is reset to 0 for unlimited retries
	if opt.MaxRetryAttempts < 0 {
		opt.MaxRetryAttempts = 0
	}

	// Create service instance with buffered close channel
	rg := &RegisterService{opt: opt, closeCh: make(chan struct{}, 1)}

	// Start registration process immediately
	return rg, rg.Register()
}

// Register establishes the etcd connection and performs initial service registration.
// It creates the etcd client, registers the service, and starts the heartbeat mechanism.
//
// Returns:
//   - error: Connection or registration errors
func (r *RegisterService) Register() (err error) {
	// Create etcd client if not already initialized
	if r.client == nil {
		config := r.opt.EtcdConfig
		r.client, err = clientv3.New(config)
		if err != nil {
			r.loggerErr("Failed to create etcd client", err)
			return err
		}
	}

	// Perform initial service registration
	if err = r.register(); err != nil {
		r.loggerErr("Service registration failed", err)
		return err
	}

	// Initialize quit notification channel
	r.quitCh = make(chan struct{}, 1)

	// Start heartbeat mechanism in background goroutine
	go r.keepAlive()

	return nil
}

// register performs the core service registration logic with etcd.
// This method handles lease creation, key-value storage, and heartbeat setup.
//
// Registration process:
// 1. Revoke any existing lease (cleanup)
// 2. Create new lease with specified TTL
// 3. Store service information with lease binding
// 4. Start lease renewal (heartbeat) mechanism
//
// Returns:
//   - error: Registration operation errors
func (r *RegisterService) register() error {
	// Clean up any existing lease to prevent resource leaks
	if r.leaseId != 0 {
		r.loggerInfo("Revoking old lease before re-registration")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		// Ignore errors as the lease might have already expired
		_, _ = r.client.Revoke(ctx, r.leaseId)
		cancel()
	}

	// Create new lease with timeout protection
	r.loggerInfo(fmt.Sprintf("Creating new lease with TTL: %d seconds", r.opt.TimeToLive))

	// Use timeout to prevent indefinite blocking on Grant operation
	grantCtx, grantCancel := context.WithTimeout(context.Background(), time.Second*10)
	defer grantCancel()

	resp, err := r.client.Grant(grantCtx, r.opt.TimeToLive)
	if err != nil {
		// Handle timeout errors by recreating the etcd client
		if errors.Is(err, context.DeadlineExceeded) {
			r.loggerErr("Grant operation timed out, will try to recreate etcd client", err)
			r.recreateEtcdClient()
		} else {
			r.loggerErr("Lease creation failed", err)
		}
		return err
	}

	r.leaseId = resp.ID
	r.loggerInfo(fmt.Sprintf("New lease created with ID: %d", r.leaseId))

	// Build the complete etcd key path
	// Format: "<Namespace>/services/<ServiceType>/<Key>/<LeaseId>"
	r.fullKey = r.buildFullKey()

	// Prepare service registration data using object pool for efficiency
	value := registerValuePool.Get().(*RegisterValue)
	defer registerValuePool.Put(value)

	// Reset object state to ensure clean data
	*value = RegisterValue{
		Timestamp: time.Now().Unix(),
		IP:        r.opt.IP,
		Port:      r.opt.Port,
		Meta:      r.opt.Meta,
	}

	// Store service information in etcd with lease binding and timeout protection
	r.loggerInfo(fmt.Sprintf("Registering service at key: %s", r.fullKey))

	putCtx, putCancel := context.WithTimeout(context.Background(), time.Second*10)
	defer putCancel()

	txn := r.client.Txn(putCtx)
	_, err = txn.Then(clientv3.OpPut(r.fullKey, value.Json(), clientv3.WithLease(r.leaseId))).Commit()
	if err != nil {
		r.loggerErr("Put operation failed", err)
		return err
	}

	// Start lease renewal mechanism (heartbeat)
	r.loggerInfo("Starting lease keep-alive mechanism")
	r.keepAliveCh, err = r.client.KeepAlive(r.client.Ctx(), r.leaseId)
	if err != nil {
		r.loggerErr("KeepAlive operation failed", err)
		return err
	}

	r.loggerInfo("Service registration completed successfully")
	return nil
}

// buildFullKey constructs the complete etcd key path for service registration.
// Uses strings.Builder with pre-allocated capacity for memory efficiency.
//
// Key format: "<Namespace>/services/<ServiceType>/<Key>/<LeaseId>"
//
// Returns:
//   - string: Complete etcd key path
func (r *RegisterService) buildFullKey() string {
	var builder strings.Builder
	// Pre-allocate capacity to avoid multiple memory allocations
	builder.Grow(len(r.opt.Namespace) + len(r.opt.ServiceType) + len(r.opt.Key) + 50)

	builder.WriteString("/")
	builder.WriteString(r.opt.Namespace)
	builder.WriteString("/services/")
	builder.WriteString(r.opt.ServiceType)
	builder.WriteString("/")
	builder.WriteString(r.opt.Key)
	builder.WriteString("/")
	builder.WriteString(strconv.FormatInt(int64(r.leaseId), 10))

	return builder.String()
}

// recreateEtcdClient handles etcd client recreation when connection issues occur.
// This method closes the existing client and creates a fresh connection with
// appropriate timeout settings to prevent blocking operations.
func (r *RegisterService) recreateEtcdClient() {
	r.loggerInfo("Recreating etcd client due to connection issues")

	// Close existing client to free resources
	if r.client != nil {
		if err := r.client.Close(); err != nil {
			r.loggerWar(fmt.Sprintf("Failed to close old etcd client: %v", err))
		}
	}

	// Create new client with timeout protection
	config := r.opt.EtcdConfig
	// Set default dial timeout if not specified
	if config.DialTimeout == 0 {
		config.DialTimeout = 5 * time.Second
	}

	client, err := clientv3.New(config)
	if err != nil {
		r.loggerErr("Failed to recreate etcd client", err)
		return
	}

	r.client = client
	r.loggerInfo("Etcd client recreated successfully")
}

// keepAlive manages the service heartbeat and automatic re-registration logic.
// This method runs in a separate goroutine and handles:
// - Lease renewal monitoring
// - Connection failure detection
// - Automatic re-registration with exponential backoff
// - Graceful shutdown coordination
func (r *RegisterService) keepAlive() {
	// Initialize retry mechanism with exponential backoff
	retryCount := 0
	maxRetries := r.opt.MaxRetryAttempts

	for {
		select {
		case <-r.closeCh:
			// Graceful shutdown requested
			r.loggerInfo("Service health check has stopped")
			return

		case res := <-r.keepAliveCh:
			// Monitor heartbeat channel for lease renewal responses
			if res == nil {
				// Heartbeat channel closed - lease has expired or connection lost
				// This can happen due to network issues, etcd restart, etc.
				r.loggerWar("Heart beat channel closed, lease may have expired")

				// Implement retry mechanism with exponential backoff
				// maxRetries = 0 means unlimited retries (recommended for production)
				for maxRetries == 0 || retryCount < maxRetries {
					retryCount++

					if maxRetries == 0 {
						r.loggerInfo(fmt.Sprintf("Attempting to re-register service (attempt %d/unlimited)", retryCount))
					} else {
						r.loggerInfo(fmt.Sprintf("Attempting to re-register service (attempt %d/%d)", retryCount, maxRetries))
					}

					// Pre-flight check: Verify etcd connectivity before attempting registration
					if !r.checkEtcdConnection() {
						r.loggerWar("etcd connection is not available, skipping registration attempt")

						// Check if maximum retry attempts reached (only when maxRetries > 0)
						if maxRetries > 0 && retryCount >= maxRetries {
							r.loggerErr("Max retry attempts reached due to etcd unavailability, service registration failed", fmt.Errorf("failed after %d attempts", maxRetries))
							// Signal service termination
							r.quitCh <- struct{}{}
							return
						}

						// Wait with exponential backoff before next connection check
						backoffTime := r.calculateBackoffTime(retryCount)
						r.loggerInfo(fmt.Sprintf("Waiting %v before next connection check", backoffTime))

						select {
						case <-r.closeCh:
							return
						case <-time.After(backoffTime):
							continue
						}
					}

					// Attempt service re-registration
					err := r.register()
					if err != nil {
						backoffTime := r.calculateBackoffTime(retryCount)
						if maxRetries == 0 {
							r.loggerErr(fmt.Sprintf("Re-registration failed (attempt %d/unlimited), retrying in %v", retryCount, backoffTime), err)
						} else {
							r.loggerErr(fmt.Sprintf("Re-registration failed (attempt %d/%d), retrying in %v", retryCount, maxRetries, backoffTime), err)
						}

						// Check if maximum retry attempts reached (only when maxRetries > 0)
						if maxRetries > 0 && retryCount >= maxRetries {
							r.loggerErr("Max retry attempts reached, service registration failed", fmt.Errorf("failed after %d attempts", maxRetries))
							// Signal service termination
							r.quitCh <- struct{}{}
							return
						}

						// Wait with exponential backoff before next retry
						select {
						case <-r.closeCh:
							return
						case <-time.After(backoffTime):
							continue
						}
					} else {
						// Re-registration successful - reset retry counter
						r.loggerInfo("Service re-registration successful")
						retryCount = 0
						break
					}
				}

				// Final check: If maxRetries > 0 and limit reached, terminate service
				if maxRetries > 0 && retryCount >= maxRetries {
					r.loggerErr("Max retry attempts reached, service registration failed", fmt.Errorf("failed after %d attempts", maxRetries))
					// Signal service termination
					r.quitCh <- struct{}{}
					return
				}
			} else {
				// Received successful heartbeat response - reset retry counter
				retryCount = 0
			}
		}
	}
}

// calculateBackoffTime implements an optimized exponential backoff strategy
// specifically designed for service discovery scenarios where fast recovery is critical.
//
// Backoff strategy:
// - Attempts 1-3: 1s, 2s, 4s (fast recovery for transient issues)
// - Attempts 4-6: 8s, 16s, 32s (moderate backoff for network issues)
// - Attempts 7+: 50s (fixed interval to balance recovery speed and resource usage)
//
// Parameters:
//   - retryCount: Current retry attempt number
//
// Returns:
//   - time.Duration: Calculated backoff duration
func (r *RegisterService) calculateBackoffTime(retryCount int) time.Duration {
	// Service discovery is a critical component, so we use an aggressive retry strategy
	// to ensure fast recovery while preventing excessive load on etcd
	switch {
	case retryCount <= 3:
		// Fast retry for transient network issues: 1s, 2s, 4s
		return time.Duration(1<<(retryCount-1)) * time.Second
	case retryCount <= 6:
		// Moderate backoff for persistent issues: 8s, 16s, 32s
		return time.Duration(1<<(retryCount-1)) * time.Second
	default:
		// Fixed interval for long-term issues to maintain regular retry attempts
		// 50 seconds provides good balance between recovery speed and resource usage
		return 50 * time.Second
	}
}

// checkEtcdConnection verifies etcd cluster connectivity by checking each endpoint.
// This method performs a lightweight status check with timeout protection.
//
// Returns:
//   - bool: true if at least one endpoint is reachable, false otherwise
func (r *RegisterService) checkEtcdConnection() bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	// Check each etcd endpoint for availability
	for _, endpoint := range r.opt.EtcdConfig.Endpoints {
		_, err := r.client.Status(ctx, endpoint)
		if err == nil {
			r.loggerInfo(fmt.Sprintf("etcd endpoint %s is available", endpoint))
			return true
		}
		r.loggerWar(fmt.Sprintf("etcd endpoint %s is not available: %v", endpoint, err))
	}

	r.loggerWar("All etcd endpoints are unavailable")
	return false
}

// Stop gracefully shuts down the service registration.
// It signals the heartbeat goroutine to stop, unregisters the service from etcd,
// and closes the etcd client connection.
//
// This method should be called during application shutdown to ensure clean cleanup.
func (r *RegisterService) Stop() {
	// Signal heartbeat goroutine to stop
	r.closeCh <- struct{}{}
	close(r.closeCh)

	// Remove service registration from etcd
	r.unregister()

	// Close etcd client connection
	if r.client != nil {
		if err := r.client.Close(); err != nil {
			r.loggerErr("etcd client failed to close", err)
		}
	}
}

// ListenQuit returns a channel that signals when the service registration
// has failed permanently and the service should terminate.
//
// This channel is triggered when:
// - Maximum retry attempts are reached (when MaxRetryAttempts > 0)
// - Unrecoverable registration errors occur
//
// Returns:
//   - <-chan struct{}: Read-only channel for quit notifications
//
// Example:
//
//	go func() {
//	    <-reg.ListenQuit()
//	    log.Println("Service registration failed, shutting down...")
//	    os.Exit(1)
//	}()
func (r *RegisterService) ListenQuit() <-chan struct{} {
	return r.quitCh
}

// unregister removes the service registration from etcd by revoking the lease
// and deleting the service key. This ensures clean removal from service discovery.
func (r *RegisterService) unregister() {
	ctx, cancel := context.WithTimeout(r.client.Ctx(), time.Second*10)
	defer cancel()

	// Revoke the lease to immediately remove the service from discovery
	if _, err := r.client.Revoke(ctx, r.leaseId); err != nil {
		r.loggerErr("unregister failed", err)
	}

	// Delete the service key as additional cleanup
	if _, err := r.client.Delete(ctx, r.fullKey); err != nil {
		r.loggerErr("unregister failed", err)
	}

	r.loggerInfo("Service has been unregister")
}

// loggerErr logs error messages using the configured logger or standard output.
// Provides consistent error logging across the service registration component.
func (r *RegisterService) loggerErr(msg string, err error) {
	if r.opt.Logger == nil {
		fmt.Printf("[service registration Error]: %s err:%s\n", msg, err.Error())
		return
	}

	r.opt.Logger.Error("[service registration]: "+msg, zap.Error(err))
}

// loggerWar logs warning messages using the configured logger or standard output.
// Used for non-critical issues that don't prevent service operation.
func (r *RegisterService) loggerWar(msg string) {
	if r.opt.Logger == nil {
		fmt.Printf("[service registration Warning]: %s \n", msg)
		return
	}

	r.opt.Logger.Warn("[service registration]: " + msg)
}

// loggerInfo logs informational messages using the configured logger or standard output.
// Used for operational status updates and debugging information.
func (r *RegisterService) loggerInfo(msg string) {
	if r.opt.Logger == nil {
		fmt.Printf("[service registration Info]: %s \n", msg)
		return
	}

	r.opt.Logger.Info("[service registration]: " + msg)
}
