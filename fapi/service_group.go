package fapi

import (
	"fmt"
	"sync"
	"time"
)

// ServiceGroup represents a collection of service instances with the same service name.
// It provides load balancing, health monitoring, and metadata management capabilities
// for a group of homogeneous service instances.
//
// ServiceGroup is thread-safe and can be safely accessed from multiple goroutines.
// It maintains service instances, applies load balancing algorithms, and tracks
// usage statistics for monitoring and debugging purposes.
type ServiceGroup struct {
	// serviceName is the logical name of the service (e.g., "user-service", "payment-api")
	serviceName string

	// services contains all registered instances of this service
	services []Service

	// loadBalancer implements the load balancing algorithm for service selection
	loadBalancer LoadBalancer

	// mu protects concurrent access to the service group's internal state
	mu sync.RWMutex

	// lastUsed tracks when this service group was last accessed for load balancing
	lastUsed time.Time

	// metadata stores arbitrary key-value pairs for service group configuration and monitoring
	metadata map[string]interface{}
}

// NewServiceGroup creates a new service group with the specified name and load balancer type.
// The service group is initialized with an empty service list and the configured load balancer.
//
// Parameters:
//   - serviceName: Logical name for the service group
//   - balancerType: Type of load balancer to use (RoundRobin, Random, etc.)
//
// Returns:
//   - *ServiceGroup: Initialized service group ready for use
//
// Example:
//
//	group := fapi.NewServiceGroup("user-service", fapi.RoundRobin)
func NewServiceGroup(serviceName string, balancerType BalancerType) *ServiceGroup {
	return &ServiceGroup{
		serviceName:  serviceName,
		services:     make([]Service, 0),
		loadBalancer: NewLoadBalancer(balancerType),
		lastUsed:     time.Now(),
		metadata:     make(map[string]interface{}),
	}
}

// AddService adds a service instance to the group.
// If a service with the same key already exists, it will be updated with the new information.
// This method is thread-safe and can be called concurrently.
//
// The method automatically updates the lastUsed timestamp to track service group activity.
//
// Parameters:
//   - service: Service instance to add or update
//
// Example:
//
//	service := Service{key: "user-service-1", value: &registerValue}
//	group.AddService(service)
func (sg *ServiceGroup) AddService(service Service) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	// Check if service already exists and update it
	for i, existingService := range sg.services {
		if existingService.key == service.key {
			sg.services[i] = service
			sg.lastUsed = time.Now()
			return
		}
	}

	// Add new service instance
	sg.services = append(sg.services, service)
	sg.lastUsed = time.Now()
}

// RemoveService removes a service instance from the group by its key.
// This method is typically called when a service instance is deregistered or becomes unavailable.
//
// Parameters:
//   - serviceKey: Unique key identifying the service instance to remove
//
// Returns:
//   - bool: true if the service was found and removed, false otherwise
//
// Example:
//
//	removed := group.RemoveService("user-service-1")
//	if removed {
//	    fmt.Println("Service instance removed successfully")
//	}
func (sg *ServiceGroup) RemoveService(serviceKey string) bool {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	for i, service := range sg.services {
		if service.key == serviceKey {
			sg.services = append(sg.services[:i], sg.services[i+1:]...)
			sg.lastUsed = time.Now()
			return true
		}
	}
	return false
}

// SelectService selects a service instance using the configured load balancing algorithm.
// This is the primary method for service selection and load distribution.
//
// The method creates a copy of the services slice to avoid holding locks during
// load balancer operations, ensuring good concurrency performance.
//
// Returns:
//   - *Service: Selected service instance
//   - error: ErrNoAvailableService if no services are available
//
// Example:
//
//	service, err := group.SelectService()
//	if err != nil {
//	    return fmt.Errorf("no service available: %w", err)
//	}
//	// Use the selected service
func (sg *ServiceGroup) SelectService() (*Service, error) {
	sg.mu.RLock()
	services := make([]Service, len(sg.services))
	copy(services, sg.services)
	sg.mu.RUnlock()

	if len(services) == 0 {
		return nil, ErrNoAvailableService
	}

	// Update last used timestamp
	sg.mu.Lock()
	sg.lastUsed = time.Now()
	sg.mu.Unlock()

	return sg.loadBalancer.Select(services)
}

// SelectServiceWithKey selects a service instance using consistent hashing with the provided key.
// This method ensures that requests with the same key are always routed to the same service instance,
// providing session affinity and supporting stateful service patterns.
//
// If the current load balancer doesn't support consistent hashing, it falls back to the
// default selection algorithm.
//
// Parameters:
//   - key: Consistent hashing key (e.g., user ID, session ID, tenant ID)
//
// Returns:
//   - *Service: Selected service instance based on consistent hashing
//   - error: ErrNoAvailableService if no services are available
//
// Example:
//
//	service, err := group.SelectServiceWithKey("user-123")
//	// Subsequent calls with "user-123" will return the same service instance
func (sg *ServiceGroup) SelectServiceWithKey(key string) (*Service, error) {
	sg.mu.RLock()
	services := make([]Service, len(sg.services))
	copy(services, sg.services)
	sg.mu.RUnlock()

	if len(services) == 0 {
		return nil, ErrNoAvailableService
	}

	// Update last used timestamp
	sg.mu.Lock()
	sg.lastUsed = time.Now()
	sg.mu.Unlock()

	// Use consistent hashing if available
	if chBalancer, ok := sg.loadBalancer.(*ConsistentHashBalancer); ok {
		return chBalancer.SelectWithKey(services, key)
	}

	// Fall back to default selection method
	return sg.loadBalancer.Select(services)
}

// SelectServiceWithIP selects a service instance using IP-based hashing.
// This provides session affinity based on client IP addresses, ensuring that
// requests from the same IP are consistently routed to the same service instance.
//
// If the current load balancer doesn't support IP hashing, it falls back to the
// default selection algorithm.
//
// Parameters:
//   - clientIP: Client IP address for hashing (e.g., "192.168.1.100")
//
// Returns:
//   - *Service: Selected service instance based on IP hashing
//   - error: ErrNoAvailableService if no services are available
//
// Example:
//
//	service, err := group.SelectServiceWithIP("192.168.1.100")
//	// All requests from this IP will be routed to the same service instance
func (sg *ServiceGroup) SelectServiceWithIP(clientIP string) (*Service, error) {
	sg.mu.RLock()
	services := make([]Service, len(sg.services))
	copy(services, sg.services)
	sg.mu.RUnlock()

	if len(services) == 0 {
		return nil, ErrNoAvailableService
	}

	// Update last used timestamp
	sg.mu.Lock()
	sg.lastUsed = time.Now()
	sg.mu.Unlock()

	// Use IP hashing if available
	if ipBalancer, ok := sg.loadBalancer.(*IPHashBalancer); ok {
		return ipBalancer.SelectWithIP(services, clientIP)
	}

	// Fall back to default selection method
	return sg.loadBalancer.Select(services)
}

// GetAllServices returns a copy of all service instances in the group.
// This method is useful for administrative purposes, monitoring, and debugging.
//
// The returned slice is a copy, so modifications to it won't affect the original service list.
//
// Returns:
//   - []Service: Copy of all service instances in the group
//
// Example:
//
//	services := group.GetAllServices()
//	for _, service := range services {
//	    fmt.Printf("Service: %s at %s\n", service.GetKey(), service.GetAddress())
//	}
func (sg *ServiceGroup) GetAllServices() []Service {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	services := make([]Service, len(sg.services))
	copy(services, sg.services)
	return services
}

// GetServiceCount returns the total number of service instances in the group.
// This includes both healthy and unhealthy instances.
//
// Returns:
//   - int: Total number of service instances
//
// Example:
//
//	count := group.GetServiceCount()
//	fmt.Printf("Service group has %d instances\n", count)
func (sg *ServiceGroup) GetServiceCount() int {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	return len(sg.services)
}

// GetServiceName returns the logical name of the service group.
//
// Returns:
//   - string: Service name (e.g., "user-service", "payment-api")
//
// Example:
//
//	name := group.GetServiceName()
//	fmt.Printf("Managing service: %s\n", name)
func (sg *ServiceGroup) GetServiceName() string {
	return sg.serviceName
}

// SetLoadBalancer dynamically changes the load balancer for this service group.
// This allows runtime reconfiguration of load balancing strategies without
// recreating the service group.
//
// Parameters:
//   - balancer: New load balancer instance to use
//
// Example:
//
//	group.SetLoadBalancer(NewLeastConnectionsBalancer())
func (sg *ServiceGroup) SetLoadBalancer(balancer LoadBalancer) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	sg.loadBalancer = balancer
}

// GetLoadBalancerName returns the name of the currently configured load balancer.
// This is useful for monitoring, debugging, and administrative interfaces.
//
// Returns:
//   - string: Name of the load balancer (e.g., "RoundRobin", "LeastConnections")
//
// Example:
//
//	balancerName := group.GetLoadBalancerName()
//	fmt.Printf("Using %s load balancer\n", balancerName)
func (sg *ServiceGroup) GetLoadBalancerName() string {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	return sg.loadBalancer.Name()
}

// ReleaseConnection releases a connection for connection-counting load balancers.
// This method is specifically designed for use with LeastConnectionsBalancer
// to properly track active connections and ensure accurate load balancing.
//
// For other load balancer types, this method has no effect.
//
// Parameters:
//   - serviceKey: Key identifying the service instance to release the connection for
//
// Example:
//
//	// After completing a request
//	group.ReleaseConnection(service.GetKey())
func (sg *ServiceGroup) ReleaseConnection(serviceKey string) {
	if lcBalancer, ok := sg.loadBalancer.(*LeastConnectionsBalancer); ok {
		lcBalancer.ReleaseConnection(serviceKey)
	}
}

// GetLastUsed returns the timestamp when this service group was last accessed.
// This information is useful for monitoring service usage patterns and
// implementing cleanup policies for unused services.
//
// Returns:
//   - time.Time: Timestamp of last access
//
// Example:
//
//	lastUsed := group.GetLastUsed()
//	if time.Since(lastUsed) > time.Hour {
//	    fmt.Println("Service group hasn't been used recently")
//	}
func (sg *ServiceGroup) GetLastUsed() time.Time {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	return sg.lastUsed
}

// SetMetadata sets a metadata key-value pair for the service group.
// Metadata can be used to store configuration, monitoring data, or any
// other information associated with the service group.
//
// Parameters:
//   - key: Metadata key
//   - value: Metadata value (can be any type)
//
// Example:
//
//	group.SetMetadata("version", "v1.2.3")
//	group.SetMetadata("region", "us-west-2")
//	group.SetMetadata("weight", 100)
func (sg *ServiceGroup) SetMetadata(key string, value interface{}) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	sg.metadata[key] = value
}

// GetMetadata retrieves a metadata value by key.
// This method returns both the value and a boolean indicating whether the key exists.
//
// Parameters:
//   - key: Metadata key to retrieve
//
// Returns:
//   - interface{}: Metadata value (nil if key doesn't exist)
//   - bool: true if key exists, false otherwise
//
// Example:
//
//	if version, exists := group.GetMetadata("version"); exists {
//	    fmt.Printf("Service version: %v\n", version)
//	}
func (sg *ServiceGroup) GetMetadata(key string) (interface{}, bool) {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	value, exists := sg.metadata[key]
	return value, exists
}

// IsEmpty checks whether the service group contains any service instances.
// This is useful for cleanup operations and determining if a service group
// should be removed from the registry.
//
// Returns:
//   - bool: true if the service group has no instances, false otherwise
//
// Example:
//
//	if group.IsEmpty() {
//	    fmt.Println("Service group is empty and can be removed")
//	}
func (sg *ServiceGroup) IsEmpty() bool {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	return len(sg.services) == 0
}

// GetHealthyServices returns only the healthy service instances from the group.
// This method filters out unhealthy services based on their health check status.
//
// Returns:
//   - []Service: Slice containing only healthy service instances
//
// Example:
//
//	healthyServices := group.GetHealthyServices()
//	fmt.Printf("Healthy instances: %d/%d\n", len(healthyServices), group.GetServiceCount())
func (sg *ServiceGroup) GetHealthyServices() []Service {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	var healthyServices []Service
	for _, service := range sg.services {
		if service.IsHealthy() {
			healthyServices = append(healthyServices, service)
		}
	}
	return healthyServices
}

// SelectHealthyService selects a service instance from only the healthy instances.
// This method applies the load balancing algorithm only to services that pass
// health checks, ensuring that traffic is not routed to unhealthy instances.
//
// Returns:
//   - *Service: Selected healthy service instance
//   - error: ErrNoAvailableService if no healthy services are available
//
// Example:
//
//	service, err := group.SelectHealthyService()
//	if err != nil {
//	    return fmt.Errorf("no healthy services available: %w", err)
//	}
//	// Use the healthy service instance
func (sg *ServiceGroup) SelectHealthyService() (*Service, error) {
	healthyServices := sg.GetHealthyServices()
	if len(healthyServices) == 0 {
		return nil, ErrNoAvailableService
	}

	// Update last used timestamp
	sg.mu.Lock()
	sg.lastUsed = time.Now()
	sg.mu.Unlock()

	return sg.loadBalancer.Select(healthyServices)
}

// String returns a human-readable string representation of the service group.
// This method is useful for logging, debugging, and monitoring purposes.
//
// Returns:
//   - string: Formatted string containing service group information
//
// Example:
//
//	fmt.Println(group.String())
//	// Output: ServiceGroup{name: user-service, instances: 3, balancer: RoundRobin, lastUsed: 2023-10-01 15:30:45}
func (sg *ServiceGroup) String() string {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	return fmt.Sprintf("ServiceGroup{name: %s, instances: %d, balancer: %s, lastUsed: %s}",
		sg.serviceName, len(sg.services), sg.loadBalancer.Name(), sg.lastUsed.Format("2006-01-02 15:04:05"))
}
