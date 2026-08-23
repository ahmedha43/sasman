package devices

import (
	"fmt"
	"strings"
	"sync"
)

var (
	driversMu sync.RWMutex
	drivers   = make(map[string]Driver)
)

// RegisterDriver associates a vendor slug with a Driver implementation
func RegisterDriver(vendorSlug string, driver Driver) {
	driversMu.Lock()
	defer driversMu.Unlock()
	slug := strings.ToLower(strings.TrimSpace(vendorSlug))
	drivers[slug] = driver
}

// GetDriver retrieves the driver implementation for a given vendor slug
func GetDriver(vendorSlug string) (Driver, error) {
	driversMu.RLock()
	defer driversMu.RUnlock()
	slug := strings.ToLower(strings.TrimSpace(vendorSlug))
	driver, exists := drivers[slug]
	if !exists {
		return nil, fmt.Errorf("no driver registered for vendor '%s'", vendorSlug)
	}
	return driver, nil
}
