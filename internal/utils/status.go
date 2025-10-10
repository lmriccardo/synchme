package utils

import (
	"slices"

	"github.com/lmriccardo/synchme/internal/proto/healthcheck"
)

// ServiceStatus is a type alias for the gRPC health check status for clarity.
type ServiceStatus healthcheck.HealthCheckResponse_ServingStatus

const (
	STATUS_UNKNOWN     = ServiceStatus(healthcheck.HealthCheckResponse_UNKNOWN)
	STATUS_SERVING     = ServiceStatus(healthcheck.HealthCheckResponse_SERVING)
	STATUS_NOT_SERVING = ServiceStatus(healthcheck.HealthCheckResponse_NOT_SERVING)

	FileSyncService    = "filesync"
	HealthCheckService = "healthcheck"
	SessionService     = "session"
)

// ServerStatus manages the health status of all registered services.
type ServerStatus struct {
	Statuses           map[string]ServiceStatus // Maps the gRPC service to its current status
	RegisteredServices []string                 // A list of all registered services
	Required           map[string]bool          // Maps each service to required or not
}

// Register registers the given service for the server health check
func (s *ServerStatus) Register(service string, required bool) {
	if !slices.Contains(s.RegisteredServices, service) {
		s.RegisteredServices = append(s.RegisteredServices, service)
		s.Required[service] = required
		s.Statuses[service] = STATUS_UNKNOWN
	}
}

// RegisterWithRequire registers a required service for the server health check
func (s *ServerStatus) RegisterWithRequire(service string) {
	s.Register(service, true)
}

// UpdateServiceStatus safely updates the status for a given service.
func (s *ServerStatus) UpdateServiceStatus(service string, status ServiceStatus) {
	if _, ok := s.Statuses[service]; !ok {
		WARN("[RPC_Client] Service ", service, " is not registered")
		return
	}

	s.Statuses[service] = status
}

// GetServiceStatus retrieves the current status of a service, defaulting to UNKNOWN if not found.
func (s *ServerStatus) GetServiceStatus(service string) ServiceStatus {
	if status, ok := s.Statuses[service]; ok {
		return status
	}
	return STATUS_UNKNOWN
}

// GetOverallStatus returns the overall status of the server. If at least one of the service
// is UNKNON or NOT SERVING, than the overall status is NOT SERVING.
func (s *ServerStatus) GetOverallStatus() ServiceStatus {
	for _, service := range s.RegisteredServices {
		if s.GetServiceStatus(service) != STATUS_SERVING && s.Required[service] {
			return STATUS_NOT_SERVING
		}
	}

	return STATUS_SERVING
}

// Clear clears old registrations
func (s *ServerStatus) Clear() {
	s.RegisteredServices = s.RegisteredServices[:0]

	for k := range s.Statuses {
		delete(s.Statuses, k)
	}

	for k := range s.Required {
		delete(s.Required, k)
	}
}
