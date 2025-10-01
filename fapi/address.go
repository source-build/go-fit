package fapi

import (
	"fmt"
	"strconv"

	"github.com/source-build/go-fit"
)

// Service represents a single service instance discovered from the service registry.
// It encapsulates service registration information including network details,
// metadata, and provides convenient methods for accessing service properties.
//
// Service instances are immutable once created and are safe for concurrent access.
type Service struct {
	// key is the unique etcd key identifying this service instance
	key string

	// value contains the service registration data including IP, port, and metadata
	value *fit.RegisterValue
}

// GetKey returns the unique etcd key for this service instance.
// The key typically follows the format: /namespace/services/type/serviceName/leaseId
//
// Returns:
//   - string: Unique service instance key
//
// Example:
//
//	key := service.GetKey()
//	fmt.Printf("Service key: %s\n", key)
func (s *Service) GetKey() string {
	return s.key
}

// GetIP returns the IP address of the service instance.
// This is the network address where the service is listening for connections.
//
// Returns:
//   - string: Service IP address, empty string if not available
//
// Example:
//
//	ip := service.GetIP()
//	fmt.Printf("Service IP: %s\n", ip)
func (s *Service) GetIP() string {
	if s.value == nil {
		return ""
	}
	return s.value.IP
}

// GetPort returns the port number of the service instance.
// This is the network port where the service is listening for connections.
//
// Returns:
//   - string: Service port number, empty string if not available
//
// Example:
//
//	port := service.GetPort()
//	fmt.Printf("Service port: %s\n", port)
func (s *Service) GetPort() string {
	if s.value == nil {
		return ""
	}
	return s.value.Port
}

// GetAddress returns the complete network address of the service instance.
// This combines the IP address and port in the format "IP:Port", which is
// ready to use for establishing network connections.
//
// Returns:
//   - string: Complete service address in "IP:Port" format, empty string if not available
//
// Example:
//
//	address := service.GetAddress()
//	conn, err := net.Dial("tcp", address)
func (s *Service) GetAddress() string {
	if s.value == nil {
		return ""
	}
	return s.value.IP + ":" + s.value.Port
}

// GetTimestamp returns the Unix timestamp when the service was registered.
// This can be used for monitoring service registration times and detecting
// service restarts or updates.
//
// Returns:
//   - int64: Unix timestamp of service registration, 0 if not available
//
// Example:
//
//	timestamp := service.GetTimestamp()
//	registrationTime := time.Unix(timestamp, 0)
//	fmt.Printf("Service registered at: %s\n", registrationTime)
func (s *Service) GetTimestamp() int64 {
	if s.value == nil {
		return 0
	}
	return s.value.Timestamp
}

// GetMeta returns the complete metadata map for the service instance.
// Metadata contains additional service information such as version, weight,
// health status, and custom application-specific data.
//
// Returns:
//   - fit.H: Service metadata map, nil if not available
//
// Example:
//
//	meta := service.GetMeta()
//	if version, exists := meta["version"]; exists {
//	    fmt.Printf("Service version: %v\n", version)
//	}
func (s *Service) GetMeta() fit.H {
	if s.value == nil {
		return nil
	}
	return s.value.Meta
}

// GetWeight returns the load balancing weight for the service instance.
// The weight determines how much traffic this instance should receive
// relative to other instances in weighted load balancing algorithms.
//
// The weight is read from the "weight" key in service metadata and supports
// int, float64, and string types. If no weight is specified or the value
// is invalid, a default weight of 1 is returned.
//
// Returns:
//   - int: Service weight (minimum 1)
//
// Example:
//
//	weight := service.GetWeight()
//	fmt.Printf("Service weight: %d\n", weight)
func (s *Service) GetWeight() int {
	if s.value == nil || s.value.Meta == nil {
		return 1
	}

	if weightVal, exists := s.value.Meta["weight"]; exists {
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

	return 1
}

// GetMetaValue retrieves a specific metadata value by key.
// This is the most flexible method for accessing service metadata,
// returning both the value and a boolean indicating whether the key exists.
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
//	if value, exists := service.GetMetaValue("region"); exists {
//	    fmt.Printf("Service region: %v\n", value)
//	}
func (s *Service) GetMetaValue(key string) (interface{}, bool) {
	if s.value == nil || s.value.Meta == nil {
		return nil, false
	}

	value, exists := s.value.Meta[key]
	return value, exists
}

// GetMetaString retrieves a string-typed metadata value by key.
// This is a convenience method that handles type assertion for string values.
// If the key doesn't exist or the value is not a string, an empty string is returned.
//
// Parameters:
//   - key: Metadata key to retrieve
//
// Returns:
//   - string: String value, empty string if key doesn't exist or value is not a string
//
// Example:
//
//	version := service.GetMetaString("version")
//	region := service.GetMetaString("region")
func (s *Service) GetMetaString(key string) string {
	if value, exists := s.GetMetaValue(key); exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// GetMetaInt retrieves an integer-typed metadata value by key.
// This method supports automatic conversion from int, float64, and string types.
// If the key doesn't exist or the value cannot be converted to an integer, 0 is returned.
//
// Parameters:
//   - key: Metadata key to retrieve
//
// Returns:
//   - int: Integer value, 0 if key doesn't exist or value cannot be converted
//
// Example:
//
//	maxConnections := service.GetMetaInt("max_connections")
//	priority := service.GetMetaInt("priority")
func (s *Service) GetMetaInt(key string) int {
	if value, exists := s.GetMetaValue(key); exists {
		switch v := value.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	return 0
}

// IsHealthy checks whether the service instance is healthy and available for traffic.
// The health status is determined by the "healthy" key in service metadata.
//
// Supported health indicator values:
// - bool: true/false
// - string: "true"/"1" for healthy, anything else for unhealthy
// - int/float64: 1 for healthy, anything else for unhealthy
//
// If no health indicator is present in metadata, the service is considered healthy by default.
//
// Returns:
//   - bool: true if service is healthy, false otherwise
//
// Example:
//
//	if service.IsHealthy() {
//	    // Route traffic to this service
//	    handleRequest(service)
//	}
func (s *Service) IsHealthy() bool {
	if healthVal, exists := s.GetMetaValue("healthy"); exists {
		switch v := healthVal.(type) {
		case bool:
			return v
		case string:
			return v == "true" || v == "1"
		case int:
			return v == 1
		case float64:
			return v == 1
		}
	}

	// Default to healthy if no health indicator is present
	return true
}

// String returns a human-readable string representation of the service instance.
// This method is useful for logging, debugging, and monitoring purposes.
//
// The string includes key service information such as the etcd key, network address,
// weight, and registration timestamp.
//
// Returns:
//   - string: Formatted string containing service information
func (s *Service) String() string {
	if s.value == nil {
		return fmt.Sprintf("Service{key: %s, value: nil}", s.key)
	}

	return fmt.Sprintf("Service{key: %s, address: %s, weight: %d, timestamp: %d}",
		s.key, s.GetAddress(), s.GetWeight(), s.value.Timestamp)
}
