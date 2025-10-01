// Package fapi provides a service discovery client for etcd-based microservice architectures.
// It offers automatic service discovery, load balancing, and health monitoring capabilities
// with support for multiple load balancing algorithms and real-time service updates.
package fapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/source-build/go-fit"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	// ErrServiceNotFound is returned when a requested service is not found in the registry.
	ErrServiceNotFound = errors.New("service not found")
)

// Options contains configuration parameters for creating a new service discovery client.
// It provides comprehensive control over etcd connection, service discovery behavior,
// and load balancing strategies.
type Options struct {
	// EtcdConfig specifies the etcd client configuration including endpoints,
	// authentication, and connection parameters.
	EtcdConfig clientv3.Config

	// PrefixKey defines the etcd key prefix for service discovery.
	// If empty, it will be automatically generated based on namespace and service type.
	// Format: "/<namespace>/services/api/"
	PrefixKey string

	// Namespace provides logical isolation for services across different environments.
	// Services in different namespaces cannot discover each other.
	// Default: "default"
	Namespace string

	// DefaultBalancerType specifies the default load balancing algorithm
	// used for services that don't have specific balancer configuration.
	// Supported types: RoundRobin, Random, WeightedRoundRobin, LeastConnections, ConsistentHash
	DefaultBalancerType BalancerType

	// ServiceBalancers allows per-service load balancer configuration.
	// Key: service name, Value: balancer type for that specific service.
	// This overrides the DefaultBalancerType for specified services.
	ServiceBalancers map[string]BalancerType
}

// Client represents a service discovery client that manages service registration
// information from etcd and provides load-balanced service selection capabilities.
// It automatically discovers services, monitors changes, and maintains service health status.
type Client struct {
	// serviceGroups stores all discovered services organized by service name.
	// Each service group contains multiple instances of the same service.
	serviceGroups map[string]*ServiceGroup

	// ctx is the root context for the client, used for cancellation and cleanup.
	ctx context.Context

	// cancel is the cancellation function for graceful shutdown.
	cancel context.CancelFunc

	// client is the etcd client instance used for all etcd operations.
	client *clientv3.Client

	// prefixKey is the etcd key prefix used for service discovery queries.
	prefixKey string

	// namespace is the logical namespace for service isolation.
	namespace string

	// mu protects concurrent access to serviceGroups map.
	mu sync.RWMutex

	// defaultBalancerType is the default load balancing algorithm.
	defaultBalancerType BalancerType

	// serviceBalancers contains per-service load balancer configurations.
	serviceBalancers map[string]BalancerType
}

// NewClient creates and initializes a new service discovery client.
// It establishes connection to etcd, discovers existing services, and starts
// real-time monitoring for service changes.
//
// Parameters:
//   - opt: Configuration options for the client
//
// Returns:
//   - *Client: Initialized service discovery client
//   - error: Configuration validation or connection errors
//
// Example:
//
//	client, err := fapi.NewClient(fapi.Options{
//	    EtcdConfig: clientv3.Config{
//	        Endpoints: []string{"localhost:2379"},
//	    },
//	    Namespace: "production",
//	    DefaultBalancerType: fapi.RoundRobin,
//	    ServiceBalancers: map[string]fapi.BalancerType{
//	        "user-service": fapi.LeastConnections,
//	    },
//	})
func NewClient(opt Options) (client *Client, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		serviceGroups:       make(map[string]*ServiceGroup),
		prefixKey:           opt.PrefixKey,
		namespace:           opt.Namespace,
		ctx:                 ctx,
		cancel:              cancel,
		defaultBalancerType: opt.DefaultBalancerType,
		serviceBalancers:    opt.ServiceBalancers,
	}

	// Apply default values for optional parameters
	if c.namespace == "" {
		c.namespace = "default"
	}

	if c.prefixKey == "" {
		c.prefixKey = fit.StringSplice("/", fit.StringSpliceTag("/", opt.Namespace, "services", "api"), "/")
	}

	if c.defaultBalancerType == "" {
		c.defaultBalancerType = RoundRobin
	}

	if c.serviceBalancers == nil {
		c.serviceBalancers = make(map[string]BalancerType)
	}

	// Create etcd client if not provided
	if c.client == nil {
		c.client, err = clientv3.New(opt.EtcdConfig)
		if err != nil {
			return nil, err
		}
	}

	return c, c.getServices()
}

// Close gracefully shuts down the service discovery client.
// It cancels all background operations and closes the etcd client connection.
// This method should be called during application shutdown to ensure clean cleanup.
func (c *Client) Close() {
	c.cancel()
	if c.client != nil {
		_ = c.client.Close()
	}
}

// getServices performs initial service discovery by querying etcd for all
// registered services under the configured prefix. It processes each service
// and starts the background watcher for real-time updates.
func (c *Client) getServices() error {
	response, err := c.client.Get(c.client.Ctx(), c.prefixKey, clientv3.WithPrefix())
	if err != nil {
		return err
	}

	// Process each discovered service
	for _, kv := range response.Kvs {
		if service, serviceName, ok := c.processKvPair(kv); ok {
			c.addServiceToGroup(serviceName, service)
		}
	}

	// Start background watcher for real-time service updates
	go c.watcher()

	return nil
}

// processKvPair processes a single etcd key-value pair and extracts service information.
// It deserializes the service registration data and creates a Service instance.
//
// Parameters:
//   - kv: etcd key-value pair containing service registration data
//
// Returns:
//   - Service: Parsed service instance
//   - string: Service name extracted from the key
//   - bool: Success indicator
func (c *Client) processKvPair(kv *mvccpb.KeyValue) (Service, string, bool) {
	var value fit.RegisterValue
	if err := json.Unmarshal(kv.Value, &value); err != nil {
		return Service{}, "", false
	}

	// Extract service name from etcd key
	// Key format: /namespace/services/api/serviceName/leaseId
	serviceName := c.extractServiceNameFromKey(string(kv.Key))
	if serviceName == "" {
		return Service{}, "", false
	}

	service := Service{
		key:   string(kv.Key),
		value: &value,
	}

	return service, serviceName, true
}

// extractServiceNameFromKey extracts the service name from an etcd key path.
// It removes the prefix and parses the remaining path to identify the service name.
//
// Key format: /namespace/services/api/serviceName/leaseId
//
// Parameters:
//   - key: Complete etcd key path
//
// Returns:
//   - string: Extracted service name, empty if parsing fails
func (c *Client) extractServiceNameFromKey(key string) string {
	// Remove the configured prefix
	if !strings.HasPrefix(key, c.prefixKey) {
		return ""
	}

	// Get the remaining path after prefix
	remaining := strings.TrimPrefix(key, c.prefixKey)

	// Split the path, expected format: serviceName/leaseId
	parts := strings.Split(remaining, "/")
	if len(parts) < 1 {
		return ""
	}

	return parts[0]
}

// addServiceToGroup adds a service instance to the appropriate service group.
// It creates a new service group if one doesn't exist for the service name,
// and configures the appropriate load balancer based on the service configuration.
//
// This method is thread-safe and handles concurrent access to the service groups.
//
// Parameters:
//   - serviceName: Name of the service
//   - service: Service instance to add
func (c *Client) addServiceToGroup(serviceName string, service Service) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Get or create service group
	serviceGroup, exists := c.serviceGroups[serviceName]
	if !exists {
		// Determine which load balancer to use
		balancerType := c.defaultBalancerType
		if specificBalancer, ok := c.serviceBalancers[serviceName]; ok {
			balancerType = specificBalancer
		}

		serviceGroup = NewServiceGroup(serviceName, balancerType)
		c.serviceGroups[serviceName] = serviceGroup
	}

	serviceGroup.AddService(service)
}

// watcher monitors etcd for real-time service registration changes.
// It runs in a background goroutine and processes service additions,
// updates, and deletions as they occur.
//
// The watcher automatically handles:
// - New service registrations (PUT events)
// - Service updates (PUT events)
// - Service deregistrations (DELETE events)
// - Graceful shutdown when the client context is cancelled
func (c *Client) watcher() {
	rch := c.client.Watch(c.client.Ctx(), c.prefixKey, clientv3.WithPrefix())
	for {
		select {
		case <-c.ctx.Done():
			return
		case v := <-rch:
			c.handlerEvents(v)
		}
	}
}

// handlerEvents processes etcd watch events and updates the service registry accordingly.
// It handles both service registration (PUT) and deregistration (DELETE) events.
//
// Parameters:
//   - resp: Watch response containing one or more etcd events
func (c *Client) handlerEvents(resp clientv3.WatchResponse) {
	for _, ev := range resp.Events {
		switch ev.Type {
		case clientv3.EventTypePut:
			// Handle service registration or update
			if service, serviceName, ok := c.processKvPair(ev.Kv); ok {
				c.addServiceToGroup(serviceName, service)
			}
		case clientv3.EventTypeDelete:
			// Handle service deregistration
			c.removeServiceByKey(string(ev.Kv.Key))
		}
	}
}

// removeServiceByKey removes a service instance from the registry based on its etcd key.
// It extracts the service name from the key and removes the specific instance.
// If the service group becomes empty after removal, it optionally deletes the group.
//
// Parameters:
//   - key: etcd key of the service instance to remove
func (c *Client) removeServiceByKey(key string) {
	serviceName := c.extractServiceNameFromKey(key)
	if serviceName == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if serviceGroup, exists := c.serviceGroups[serviceName]; exists {
		serviceGroup.RemoveService(key)

		// Optionally remove empty service groups to prevent memory leaks
		if serviceGroup.IsEmpty() {
			delete(c.serviceGroups, serviceName)
		}
	}
}

// SelectService selects a service instance using the configured load balancing algorithm.
// This is the primary method for service discovery and load balancing.
//
// Parameters:
//   - serviceName: Name of the service to select from
//
// Returns:
//   - *Service: Selected service instance
//   - error: ErrServiceNotFound if the service doesn't exist, or load balancer errors
//
// Example:
//
//	service, err := client.SelectService("user-service")
//	if err != nil {
//	    return err
//	}
//	// Use service.GetAddress() to get the service endpoint
func (c *Client) SelectService(serviceName string) (*Service, error) {
	c.mu.RLock()
	serviceGroup, exists := c.serviceGroups[serviceName]
	c.mu.RUnlock()

	if !exists {
		return nil, ErrServiceNotFound
	}

	return serviceGroup.SelectService()
}

// SelectServiceWithKey selects a service instance using consistent hashing with the provided key.
// This ensures that requests with the same key are always routed to the same service instance,
// which is useful for session affinity and stateful services.
//
// Parameters:
//   - serviceName: Name of the service to select from
//   - key: Consistent hashing key (e.g., user ID, session ID)
//
// Returns:
//   - *Service: Selected service instance based on consistent hashing
//   - error: ErrServiceNotFound if the service doesn't exist, or load balancer errors
//
// Example:
//
//	service, err := client.SelectServiceWithKey("user-service", userID)
func (c *Client) SelectServiceWithKey(serviceName, key string) (*Service, error) {
	c.mu.RLock()
	serviceGroup, exists := c.serviceGroups[serviceName]
	c.mu.RUnlock()

	if !exists {
		return nil, ErrServiceNotFound
	}

	return serviceGroup.SelectServiceWithKey(key)
}

// SelectServiceWithIP selects a service instance using IP-based hashing.
// This provides a form of session affinity based on client IP addresses,
// ensuring that requests from the same IP are routed to the same service instance.
//
// Parameters:
//   - serviceName: Name of the service to select from
//   - clientIP: Client IP address for hashing
//
// Returns:
//   - *Service: Selected service instance based on IP hashing
//   - error: ErrServiceNotFound if the service doesn't exist, or load balancer errors
//
// Example:
//
//	service, err := client.SelectServiceWithIP("user-service", "192.168.1.100")
func (c *Client) SelectServiceWithIP(serviceName, clientIP string) (*Service, error) {
	c.mu.RLock()
	serviceGroup, exists := c.serviceGroups[serviceName]
	c.mu.RUnlock()

	if !exists {
		return nil, ErrServiceNotFound
	}

	return serviceGroup.SelectServiceWithIP(clientIP)
}

// SelectHealthyService selects a service instance from only healthy instances.
// This method filters out unhealthy services and applies load balancing
// only to the remaining healthy instances.
//
// Parameters:
//   - serviceName: Name of the service to select from
//
// Returns:
//   - *Service: Selected healthy service instance
//   - error: ErrServiceNotFound if the service doesn't exist, or if no healthy instances are available
//
// Example:
//
//	service, err := client.SelectHealthyService("user-service")
func (c *Client) SelectHealthyService(serviceName string) (*Service, error) {
	c.mu.RLock()
	serviceGroup, exists := c.serviceGroups[serviceName]
	c.mu.RUnlock()

	if !exists {
		return nil, ErrServiceNotFound
	}

	return serviceGroup.SelectHealthyService()
}

// GetAllServices returns all instances of a specific service.
// This is useful for administrative purposes, monitoring, or when you need
// to interact with all instances of a service.
//
// Parameters:
//   - serviceName: Name of the service
//
// Returns:
//   - []Service: All instances of the specified service
//   - error: ErrServiceNotFound if the service doesn't exist
//
// Example:
//
//	services, err := client.GetAllServices("user-service")
//	for _, service := range services {
//	    fmt.Printf("Service instance: %s\n", service.GetAddress())
//	}
func (c *Client) GetAllServices(serviceName string) ([]Service, error) {
	c.mu.RLock()
	serviceGroup, exists := c.serviceGroups[serviceName]
	c.mu.RUnlock()

	if !exists {
		return nil, ErrServiceNotFound
	}

	return serviceGroup.GetAllServices(), nil
}

// GetServiceCount returns the number of instances for a specific service.
// This includes both healthy and unhealthy instances.
//
// Parameters:
//   - serviceName: Name of the service
//
// Returns:
//   - int: Number of service instances, 0 if service doesn't exist
//
// Example:
//
//	count := client.GetServiceCount("user-service")
//	fmt.Printf("User service has %d instances\n", count)
func (c *Client) GetServiceCount(serviceName string) int {
	c.mu.RLock()
	serviceGroup, exists := c.serviceGroups[serviceName]
	c.mu.RUnlock()

	if !exists {
		return 0
	}

	return serviceGroup.GetServiceCount()
}

// GetAllServiceNames returns the names of all discovered services.
// This is useful for service discovery, monitoring dashboards, and administrative tools.
//
// Returns:
//   - []string: List of all service names currently in the registry
//
// Example:
//
//	serviceNames := client.GetAllServiceNames()
//	for _, name := range serviceNames {
//	    fmt.Printf("Discovered service: %s\n", name)
//	}
func (c *Client) GetAllServiceNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	serviceNames := make([]string, 0, len(c.serviceGroups))
	for serviceName := range c.serviceGroups {
		serviceNames = append(serviceNames, serviceName)
	}
	return serviceNames
}

// GetServiceGroup returns the service group for a specific service.
// This provides direct access to the service group for advanced operations.
//
// Parameters:
//   - serviceName: Name of the service
//
// Returns:
//   - *ServiceGroup: Service group containing all instances of the service
//   - error: ErrServiceNotFound if the service doesn't exist
//
// Example:
//
//	group, err := client.GetServiceGroup("user-service")
//	if err != nil {
//	    return err
//	}
//
// Access advanced service group methods
func (c *Client) GetServiceGroup(serviceName string) (*ServiceGroup, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	serviceGroup, exists := c.serviceGroups[serviceName]
	if !exists {
		return nil, ErrServiceNotFound
	}

	return serviceGroup, nil
}

// SetServiceLoadBalancer dynamically changes the load balancer for a specific service.
// This allows runtime reconfiguration of load balancing strategies without restarting the client.
//
// Parameters:
//   - serviceName: Name of the service
//   - balancer: New load balancer instance to use
//
// Returns:
//   - error: ErrServiceNotFound if the service doesn't exist
//
// Example:
//
//	err := client.SetServiceLoadBalancer("user-service", NewLeastConnectionsBalancer())
func (c *Client) SetServiceLoadBalancer(serviceName string, balancer LoadBalancer) error {
	c.mu.RLock()
	serviceGroup, exists := c.serviceGroups[serviceName]
	c.mu.RUnlock()

	if !exists {
		return ErrServiceNotFound
	}

	serviceGroup.SetLoadBalancer(balancer)
	return nil
}

// GetServiceLoadBalancerName returns the name of the load balancer currently used by a service.
// This is useful for monitoring and debugging load balancing behavior.
//
// Parameters:
//   - serviceName: Name of the service
//
// Returns:
//   - string: Name of the load balancer (e.g., "RoundRobin", "LeastConnections")
//   - error: ErrServiceNotFound if the service doesn't exist
//
// Example:
//
//	balancerName, err := client.GetServiceLoadBalancerName("user-service")
//	fmt.Printf("User service uses %s load balancer\n", balancerName)
func (c *Client) GetServiceLoadBalancerName(serviceName string) (string, error) {
	c.mu.RLock()
	serviceGroup, exists := c.serviceGroups[serviceName]
	c.mu.RUnlock()

	if !exists {
		return "", ErrServiceNotFound
	}

	return serviceGroup.GetLoadBalancerName(), nil
}

// ReleaseConnection releases a connection for connection-counting load balancers.
// This is specifically used with LeastConnections load balancer to properly
// track connection counts and ensure accurate load balancing.
//
// Parameters:
//   - serviceName: Name of the service
//   - serviceKey: Key identifying the specific service instance
//
// Returns:
//   - error: ErrServiceNotFound if the service doesn't exist
//
// Example:
//
//	// After completing a request to a service
//	err := client.ReleaseConnection("user-service", service.GetKey())
func (c *Client) ReleaseConnection(serviceName, serviceKey string) error {
	c.mu.RLock()
	serviceGroup, exists := c.serviceGroups[serviceName]
	c.mu.RUnlock()

	if !exists {
		return ErrServiceNotFound
	}

	serviceGroup.ReleaseConnection(serviceKey)
	return nil
}

// GetServiceGroupsStatus returns comprehensive status information for all service groups.
// This provides a complete overview of the service registry state, including
// instance counts, load balancer types, and health statistics.
//
// Returns:
//   - map[string]ServiceGroupStatus: Status information keyed by service name
//
// Example:
//
//	status := client.GetServiceGroupsStatus()
//	for serviceName, info := range status {
//	    fmt.Printf("Service: %s, Instances: %d, Healthy: %d, Balancer: %s\n",
//	        serviceName, info.InstanceCount, info.HealthyCount, info.BalancerName)
//	}
func (c *Client) GetServiceGroupsStatus() map[string]ServiceGroupStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := make(map[string]ServiceGroupStatus)
	for serviceName, serviceGroup := range c.serviceGroups {
		status[serviceName] = ServiceGroupStatus{
			ServiceName:   serviceName,
			InstanceCount: serviceGroup.GetServiceCount(),
			BalancerName:  serviceGroup.GetLoadBalancerName(),
			LastUsed:      serviceGroup.GetLastUsed(),
			HealthyCount:  len(serviceGroup.GetHealthyServices()),
		}
	}
	return status
}

// ServiceGroupStatus contains comprehensive status information for a service group.
// It provides insights into service health, usage patterns, and configuration.
type ServiceGroupStatus struct {
	// ServiceName is the name of the service
	ServiceName string `json:"service_name"`

	// InstanceCount is the total number of service instances (healthy + unhealthy)
	InstanceCount int `json:"instance_count"`

	// BalancerName is the name of the load balancer currently in use
	BalancerName string `json:"balancer_name"`

	// LastUsed indicates when the service group was last accessed for load balancing
	LastUsed time.Time `json:"last_used"`

	// HealthyCount is the number of healthy service instances
	HealthyCount int `json:"healthy_count"`
}

// HasService checks whether a specific service exists in the registry.
// This is a lightweight operation that only checks for service existence
// without considering instance count or health status.
//
// Parameters:
//   - serviceName: Name of the service to check
//
// Returns:
//   - bool: true if the service exists, false otherwise
//
// Example:
//
//	if client.HasService("user-service") {
//	    fmt.Println("User service is available")
//	}
func (c *Client) HasService(serviceName string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, exists := c.serviceGroups[serviceName]
	return exists
}

// WaitForService waits for a specific service to become available with a timeout.
// This is useful during application startup when you need to ensure that
// dependent services are available before proceeding.
//
// The method checks both service existence and that at least one instance is available.
//
// Parameters:
//   - serviceName: Name of the service to wait for
//   - timeout: Maximum time to wait for the service
//
// Returns:
//   - error: Timeout error if service doesn't become available within the specified time
//
// Example:
//
//	// Wait up to 30 seconds for user service to be available
//	err := client.WaitForService("user-service", 30*time.Second)
//	if err != nil {
//	    log.Fatal("User service not available:", err)
//	}
func (c *Client) WaitForService(serviceName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if c.HasService(serviceName) && c.GetServiceCount(serviceName) > 0 {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("service %s not available within timeout", serviceName)
}
