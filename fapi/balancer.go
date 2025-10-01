package fapi

import (
	"errors"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrNoAvailableService is returned when no service instances are available for selection.
	ErrNoAvailableService = errors.New("no available service instances")
)

// LoadBalancer defines the interface for load balancing algorithms.
// Implementations of this interface provide different strategies for selecting
// service instances from a pool of available services.
//
// All load balancer implementations must be thread-safe as they may be
// accessed concurrently from multiple goroutines.
type LoadBalancer interface {
	// Select chooses a service instance from the provided list using the
	// load balancer's specific algorithm.
	//
	// Parameters:
	//   - services: List of available service instances
	//
	// Returns:
	//   - *Service: Selected service instance
	//   - error: ErrNoAvailableService if no services are available
	Select(services []Service) (*Service, error)

	// Name returns the human-readable name of the load balancer algorithm.
	// This is used for monitoring, logging, and administrative purposes.
	//
	// Returns:
	//   - string: Load balancer name (e.g., "round_robin", "random")
	Name() string
}

// RoundRobinBalancer implements a round-robin load balancing algorithm.
// It distributes requests evenly across all available service instances
// by cycling through them in order.
//
// This balancer is thread-safe and uses atomic operations to ensure
// correct behavior under concurrent access.
type RoundRobinBalancer struct {
	// counter tracks the current position in the round-robin cycle
	counter uint64
}

// NewRoundRobinBalancer creates a new round-robin load balancer instance.
//
// Returns:
//   - *RoundRobinBalancer: Initialized round-robin balancer
//
// Example:
//
//	balancer := fapi.NewRoundRobinBalancer()
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

// Select implements the LoadBalancer interface for round-robin selection.
// It uses atomic operations to ensure thread-safe counter incrementation.
//
// The algorithm cycles through services in order: 0, 1, 2, ..., n-1, 0, 1, ...
//
// Parameters:
//   - services: List of available service instances
//
// Returns:
//   - *Service: Next service in the round-robin cycle
//   - error: ErrNoAvailableService if services list is empty
func (r *RoundRobinBalancer) Select(services []Service) (*Service, error) {
	if len(services) == 0 {
		return nil, ErrNoAvailableService
	}

	// Use atomic operations to ensure thread safety
	index := atomic.AddUint64(&r.counter, 1) % uint64(len(services))
	return &services[index], nil
}

// Name returns the identifier for the round-robin load balancer.
func (r *RoundRobinBalancer) Name() string {
	return "round_robin"
}

// RandomBalancer implements a random load balancing algorithm.
// It selects service instances randomly from the available pool,
// providing good distribution over time while avoiding predictable patterns.
//
// This balancer uses a separate random number generator per instance
// to ensure thread safety and good randomness.
type RandomBalancer struct {
	// rand is the random number generator instance
	rand *rand.Rand
	// mu protects access to the random number generator
	mu sync.Mutex
}

// NewRandomBalancer creates a new random load balancer instance.
// Each instance uses its own seeded random number generator for thread safety.
//
// Returns:
//   - *RandomBalancer: Initialized random balancer
//
// Example:
//
//	balancer := fapi.NewRandomBalancer()
func NewRandomBalancer() *RandomBalancer {
	return &RandomBalancer{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Select implements the LoadBalancer interface for random selection.
// It uses a mutex-protected random number generator to ensure thread safety.
//
// Parameters:
//   - services: List of available service instances
//
// Returns:
//   - *Service: Randomly selected service instance
//   - error: ErrNoAvailableService if services list is empty
func (r *RandomBalancer) Select(services []Service) (*Service, error) {
	if len(services) == 0 {
		return nil, ErrNoAvailableService
	}

	r.mu.Lock()
	index := r.rand.Intn(len(services))
	r.mu.Unlock()

	return &services[index], nil
}

// Name returns the identifier for the random load balancer.
func (r *RandomBalancer) Name() string {
	return "random"
}

// WeightedRoundRobinBalancer implements a weighted round-robin load balancing algorithm.
// It distributes requests based on service weights, ensuring that services with
// higher weights receive proportionally more traffic.
//
// Service weights are read from the service metadata under the "weight" key.
// If no weight is specified, a default weight of 1 is used.
//
// The algorithm maintains current weights for each service and selects the
// service with the highest current weight, then adjusts weights accordingly.
type WeightedRoundRobinBalancer struct {
	// mu protects access to currentWeights map
	mu sync.Mutex
	// currentWeights tracks the current weight for each service instance
	currentWeights map[string]int
}

// NewWeightedRoundRobinBalancer creates a new weighted round-robin load balancer.
//
// Returns:
//   - *WeightedRoundRobinBalancer: Initialized weighted round-robin balancer
//
// Example:
//
//	balancer := fapi.NewWeightedRoundRobinBalancer()
func NewWeightedRoundRobinBalancer() *WeightedRoundRobinBalancer {
	return &WeightedRoundRobinBalancer{
		currentWeights: make(map[string]int),
	}
}

// Select implements the LoadBalancer interface for weighted round-robin selection.
// The algorithm:
// 1. Increases each service's current weight by its configured weight
// 2. Selects the service with the highest current weight
// 3. Decreases the selected service's current weight by the total weight
//
// Parameters:
//   - services: List of available service instances
//
// Returns:
//   - *Service: Service selected based on weighted round-robin algorithm
//   - error: ErrNoAvailableService if services list is empty
func (w *WeightedRoundRobinBalancer) Select(services []Service) (*Service, error) {
	if len(services) == 0 {
		return nil, ErrNoAvailableService
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	var selected *Service
	totalWeight := 0

	// Calculate total weight and update current weights
	for i := range services {
		service := &services[i]
		weight := w.getServiceWeight(service)
		totalWeight += weight

		// Increase current weight
		w.currentWeights[service.key] += weight

		// Select service with highest current weight
		if selected == nil || w.currentWeights[service.key] > w.currentWeights[selected.key] {
			selected = service
		}
	}

	if selected != nil {
		// Decrease selected service's current weight
		w.currentWeights[selected.key] -= totalWeight
	}

	return selected, nil
}

// getServiceWeight extracts the weight value from service metadata.
// It supports weight values as int, float64, or string types.
// Returns 1 as the default weight if no valid weight is found.
//
// Parameters:
//   - service: Service instance to get weight for
//
// Returns:
//   - int: Service weight (minimum 1)
func (w *WeightedRoundRobinBalancer) getServiceWeight(service *Service) int {
	if service.value.Meta == nil {
		return 1 // Default weight
	}

	if weightVal, exists := service.value.Meta["weight"]; exists {
		switch v := weightVal.(type) {
		case int:
			if v > 0 {
				return v
			}
		case float64:
			if v > 0 {
				return int(v)
			}
		case string:
			if weight, err := strconv.Atoi(v); err == nil && weight > 0 {
				return weight
			}
		}
	}

	return 1 // Default weight
}

// Name returns the identifier for the weighted round-robin load balancer.
func (w *WeightedRoundRobinBalancer) Name() string {
	return "weighted_round_robin"
}

// LeastConnectionsBalancer implements a least connections load balancing algorithm.
// It selects the service instance with the fewest active connections,
// helping to distribute load more evenly when connection durations vary.
//
// Note: This implementation maintains simulated connection counts.
// In production use, you should integrate with actual connection tracking.
type LeastConnectionsBalancer struct {
	// mu protects access to connections map
	mu sync.Mutex
	// connections tracks the number of active connections per service
	connections map[string]int64
}

// NewLeastConnectionsBalancer creates a new least connections load balancer.
//
// Returns:
//   - *LeastConnectionsBalancer: Initialized least connections balancer
//
// Example:
//
//	balancer := fapi.NewLeastConnectionsBalancer()
func NewLeastConnectionsBalancer() *LeastConnectionsBalancer {
	return &LeastConnectionsBalancer{
		connections: make(map[string]int64),
	}
}

// Select implements the LoadBalancer interface for least connections selection.
// It selects the service instance with the minimum number of active connections
// and increments the connection count for the selected service.
//
// Parameters:
//   - services: List of available service instances
//
// Returns:
//   - *Service: Service with the least connections
//   - error: ErrNoAvailableService if services list is empty
func (l *LeastConnectionsBalancer) Select(services []Service) (*Service, error) {
	if len(services) == 0 {
		return nil, ErrNoAvailableService
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	var selected *Service
	minConnections := int64(-1)

	for i := range services {
		service := &services[i]
		connections := l.connections[service.key]

		if minConnections == -1 || connections < minConnections {
			minConnections = connections
			selected = service
		}
	}

	if selected != nil {
		// Increment connection count (simulated)
		l.connections[selected.key]++
	}

	return selected, nil
}

// ReleaseConnection decrements the connection count for a service instance.
// This method should be called when a connection to a service is closed
// to maintain accurate connection counts for load balancing decisions.
//
// Parameters:
//   - serviceKey: Key identifying the service instance
//
// Example:
//
//	// After completing a request
//	balancer.ReleaseConnection(service.GetKey())
func (l *LeastConnectionsBalancer) ReleaseConnection(serviceKey string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.connections[serviceKey] > 0 {
		l.connections[serviceKey]--
	}
}

// Name returns the identifier for the least connections load balancer.
func (l *LeastConnectionsBalancer) Name() string {
	return "least_connections"
}

// ConsistentHashBalancer implements a consistent hashing load balancing algorithm.
// It ensures that requests with the same key are always routed to the same
// service instance, providing session affinity and supporting stateful services.
//
// This implementation uses a simple hash function and modulo operation.
// For production use, consider implementing a proper consistent hash ring
// with virtual nodes for better distribution.
type ConsistentHashBalancer struct {
	// hashFunc is the hash function used for key hashing
	hashFunc func(data []byte) uint32
}

// NewConsistentHashBalancer creates a new consistent hash load balancer.
//
// Returns:
//   - *ConsistentHashBalancer: Initialized consistent hash balancer
//
// Example:
//
//	balancer := fapi.NewConsistentHashBalancer()
func NewConsistentHashBalancer() *ConsistentHashBalancer {
	return &ConsistentHashBalancer{
		hashFunc: defaultHash,
	}
}

// Select implements the LoadBalancer interface for consistent hashing.
// This method uses the current time as a key, which provides random distribution.
// For actual consistent hashing, use SelectWithKey method instead.
//
// Parameters:
//   - services: List of available service instances
//
// Returns:
//   - *Service: Service selected based on time-based hash
//   - error: ErrNoAvailableService if services list is empty
func (c *ConsistentHashBalancer) Select(services []Service) (*Service, error) {
	if len(services) == 0 {
		return nil, ErrNoAvailableService
	}

	// Simple consistent hash implementation
	// In actual use, you should pass a key for hashing
	// Using current time as an example
	key := time.Now().String()
	hash := c.hashFunc([]byte(key))
	index := hash % uint32(len(services))

	return &services[index], nil
}

// SelectWithKey selects a service instance using consistent hashing with the provided key.
// This ensures that the same key always maps to the same service instance,
// providing session affinity and supporting stateful service patterns.
//
// Parameters:
//   - services: List of available service instances
//   - key: Key to use for consistent hashing (e.g., user ID, session ID)
//
// Returns:
//   - *Service: Service selected based on key hash
//   - error: ErrNoAvailableService if services list is empty
//
// Example:
//
//	service, err := balancer.SelectWithKey(services, "user-123")
//	// Subsequent calls with "user-123" will return the same service
func (c *ConsistentHashBalancer) SelectWithKey(services []Service, key string) (*Service, error) {
	if len(services) == 0 {
		return nil, ErrNoAvailableService
	}

	hash := c.hashFunc([]byte(key))
	index := hash % uint32(len(services))

	return &services[index], nil
}

// Name returns the identifier for the consistent hash load balancer.
func (c *ConsistentHashBalancer) Name() string {
	return "consistent_hash"
}

// defaultHash implements a simple FNV-1a hash function.
// This provides good distribution for most use cases.
// For cryptographic security, consider using a different hash function.
//
// Parameters:
//   - data: Data to hash
//
// Returns:
//   - uint32: Hash value
func defaultHash(data []byte) uint32 {
	hash := uint32(2166136261)
	for _, b := range data {
		hash ^= uint32(b)
		hash *= 16777619
	}
	return hash
}

// IPHashBalancer implements an IP-based hash load balancing algorithm.
// It routes requests from the same client IP to the same service instance,
// providing a form of session affinity based on client location.
//
// This balancer is useful for scenarios where you want to maintain
// some level of stickiness without requiring explicit session management.
type IPHashBalancer struct {
	// hashFunc is the hash function used for IP hashing
	hashFunc func(data []byte) uint32
}

// NewIPHashBalancer creates a new IP hash load balancer.
//
// Returns:
//   - *IPHashBalancer: Initialized IP hash balancer
//
// Example:
//
//	balancer := fapi.NewIPHashBalancer()
func NewIPHashBalancer() *IPHashBalancer {
	return &IPHashBalancer{
		hashFunc: defaultHash,
	}
}

// Select implements the LoadBalancer interface but returns an error for IP hash balancer.
// This balancer requires the client IP address, so use SelectWithIP method instead.
//
// Parameters:
//   - services: List of available service instances (unused)
//
// Returns:
//   - *Service: Always nil
//   - error: Error indicating that SelectWithIP should be used
func (i *IPHashBalancer) Select(services []Service) (*Service, error) {
	return nil, errors.New("IPHashBalancer requires SelectWithIP method")
}

// SelectWithIP selects a service instance using IP-based hashing.
// It ensures that requests from the same client IP are always routed
// to the same service instance.
//
// Parameters:
//   - services: List of available service instances
//   - clientIP: Client IP address for hashing (e.g., "192.168.1.100")
//
// Returns:
//   - *Service: Service selected based on IP hash
//   - error: ErrNoAvailableService if services list is empty
//
// Example:
//
//	service, err := balancer.SelectWithIP(services, "192.168.1.100")
//	// All requests from this IP will be routed to the same service
func (i *IPHashBalancer) SelectWithIP(services []Service, clientIP string) (*Service, error) {
	if len(services) == 0 {
		return nil, ErrNoAvailableService
	}

	hash := i.hashFunc([]byte(clientIP))
	index := hash % uint32(len(services))

	return &services[index], nil
}

// Name returns the identifier for the IP hash load balancer.
func (i *IPHashBalancer) Name() string {
	return "ip_hash"
}

// BalancerType represents the type of load balancing algorithm.
// It provides a type-safe way to specify load balancer types in configuration.
type BalancerType string

const (
	// RoundRobin distributes requests evenly across all service instances
	RoundRobin BalancerType = "round_robin"

	// Random selects service instances randomly
	Random BalancerType = "random"

	// WeightedRoundRobin distributes requests based on service weights
	WeightedRoundRobin BalancerType = "weighted_round_robin"

	// LeastConnections selects the service with the fewest active connections
	LeastConnections BalancerType = "least_connections"

	// ConsistentHash provides session affinity using consistent hashing
	ConsistentHash BalancerType = "consistent_hash"

	// IPHash provides session affinity based on client IP addresses
	IPHash BalancerType = "ip_hash"
)

// NewLoadBalancer creates a load balancer instance based on the specified type.
// This factory function provides a convenient way to create load balancers
// without directly instantiating specific implementations.
//
// Parameters:
//   - balancerType: Type of load balancer to create
//
// Returns:
//   - LoadBalancer: Initialized load balancer instance
//
// Example:
//
//	balancer := fapi.NewLoadBalancer(fapi.RoundRobin)
//	service, err := balancer.Select(services)
func NewLoadBalancer(balancerType BalancerType) LoadBalancer {
	switch balancerType {
	case RoundRobin:
		return NewRoundRobinBalancer()
	case Random:
		return NewRandomBalancer()
	case WeightedRoundRobin:
		return NewWeightedRoundRobinBalancer()
	case LeastConnections:
		return NewLeastConnectionsBalancer()
	case ConsistentHash:
		return NewConsistentHashBalancer()
	case IPHash:
		return NewIPHashBalancer()
	default:
		return NewRoundRobinBalancer() // Default to round-robin
	}
}
