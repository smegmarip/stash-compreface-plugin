package config

import (
	"fmt"
	"net"
	"net/url"

	"github.com/stashapp/stash/pkg/plugin/common"
	"github.com/stashapp/stash/pkg/plugin/common/log"
)

// Load loads and validates plugin configuration from Stash settings
func Load(input common.PluginInput) (*PluginConfig, error) {
	config := &PluginConfig{
		// Default values
		CooldownSeconds: 10,
		MaxBatchSize:    20,
		MinSimilarity:   0.89,
		MinFaceSize:     64,
		ScannedTagName:  "Compreface Scanned",
		MatchedTagName:  "Compreface Matched",
	}

	// Fetch plugin configuration from Stash
	pluginConfig, err := getPluginConfiguration()
	if err != nil {
		log.Warnf("Failed to get plugin configuration: %v, using defaults", err)
		// Don't fail - use defaults
	} else {
		// Override defaults with user settings
		if val := getStringSetting(pluginConfig, "comprefaceUrl"); val != "" {
			config.ComprefaceURL = val
		}
		if val := getStringSetting(pluginConfig, "recognitionApiKey"); val != "" {
			config.RecognitionAPIKey = val
		}
		if val := getStringSetting(pluginConfig, "detectionApiKey"); val != "" {
			config.DetectionAPIKey = val
		}
		if val := getStringSetting(pluginConfig, "verificationApiKey"); val != "" {
			config.VerificationAPIKey = val
		}
		if val := getIntSetting(pluginConfig, "cooldownSeconds"); val > 0 {
			config.CooldownSeconds = val
		}
		if val := getIntSetting(pluginConfig, "maxBatchSize"); val > 0 {
			config.MaxBatchSize = val
		}
		if val := getFloatSetting(pluginConfig, "minSimilarity"); val > 0 {
			config.MinSimilarity = val
		}
		if val := getIntSetting(pluginConfig, "minFaceSize"); val > 0 {
			config.MinFaceSize = val
		}
		if val := getStringSetting(pluginConfig, "scannedTagName"); val != "" {
			config.ScannedTagName = val
		}
		if val := getStringSetting(pluginConfig, "matchedTagName"); val != "" {
			config.MatchedTagName = val
		}
		if val := getStringSetting(pluginConfig, "visionServiceUrl"); val != "" {
			config.VisionServiceURL = val
		}
		if val := getStringSetting(pluginConfig, "qualityServiceUrl"); val != "" {
			config.QualityServiceURL = val
		}
	}

	// Resolve Compreface URL with auto-detection
	config.ComprefaceURL = resolveServiceURL(config.ComprefaceURL, "compreface", "8000")

	// Resolve Vision Service URL with auto-detection (optional service)
	if config.VisionServiceURL != "" {
		config.VisionServiceURL = resolveServiceURL(config.VisionServiceURL, "stash-auto-vision", "5000")
		log.Infof("Vision Service configured at: %s", config.VisionServiceURL)
	} else {
		log.Info("Vision Service not configured (video recognition disabled)")
	}

	// Resolve Quality Service URL with auto-detection (optional service)
	if config.QualityServiceURL != "" {
		config.QualityServiceURL = resolveServiceURL(config.QualityServiceURL, "stash-face-quality", "6001")
		log.Infof("Quality Service configured at: %s", config.QualityServiceURL)
	} else {
		log.Info("Quality Service not configured (enhanced quality assessment disabled)")
	}

	// Validate required settings
	if config.RecognitionAPIKey == "" {
		return nil, fmt.Errorf("recognition API key is required")
	}
	if config.DetectionAPIKey == "" {
		return nil, fmt.Errorf("detection API key is required")
	}

	return config, nil
}

// getPluginConfiguration fetches plugin configuration from Stash via GraphQL
// NOTE: This function is currently disabled since it requires graphqlClient access
// Configuration loading is handled differently in the RPC service
func getPluginConfiguration() (map[string]interface{}, error) {
	// TODO: Implement configuration fetching via server connection
	// For now, return empty config to use defaults
	log.Debug("Plugin configuration fetching not yet implemented, using defaults")
	return make(map[string]interface{}), nil
}

// getStringSetting retrieves a string setting from plugin config
func getStringSetting(config map[string]interface{}, key string) string {
	if val, ok := config[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getIntSetting retrieves an integer setting from plugin config
func getIntSetting(config map[string]interface{}, key string) int {
	if val, ok := config[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}

// getFloatSetting retrieves a float setting from plugin config
func getFloatSetting(config map[string]interface{}, key string) float64 {
	if val, ok := config[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return 0.0
}

// resolveServiceURL resolves the service URL with proper DNS lookup.
// Handles IP addresses, hostnames, container names, and localhost.
//
// Based on auto-caption pattern for Docker Compose compatibility.
//
// Parameters:
//   - configuredURL: The URL from configuration (may be empty)
//   - defaultContainerName: Default container name for auto-detection
//   - defaultPort: Default port number
//
// Returns: Resolved URL
func resolveServiceURL(configuredURL string, defaultContainerName string, defaultPort string) string {
	const defaultScheme = "http"
	var hardcodedFallback = fmt.Sprintf("%s://%s:%s", defaultScheme, defaultContainerName, defaultPort)

	// If no URL configured, use fallback
	if configuredURL == "" {
		log.Infof("No service URL configured, using default: %s", hardcodedFallback)
		return hardcodedFallback
	}

	// Parse the URL
	parsedURL, err := url.Parse(configuredURL)
	if err != nil {
		log.Warnf("Failed to parse service URL '%s': %v, using fallback", configuredURL, err)
		return hardcodedFallback
	}

	hostname := parsedURL.Hostname()
	port := parsedURL.Port()
	scheme := parsedURL.Scheme

	// Default scheme if not specified
	if scheme == "" {
		scheme = defaultScheme
	}

	// Default port if not specified
	if port == "" {
		port = defaultPort
	}

	// Case 1: localhost - use as-is
	if hostname == "localhost" || hostname == "127.0.0.1" {
		resolvedURL := fmt.Sprintf("%s://%s:%s", scheme, hostname, port)
		log.Infof("Using localhost service URL: %s", resolvedURL)
		return resolvedURL
	}

	// Case 2: Already an IP address - use as-is
	if net.ParseIP(hostname) != nil {
		resolvedURL := fmt.Sprintf("%s://%s:%s", scheme, hostname, port)
		log.Infof("Using IP-based service URL: %s", resolvedURL)
		return resolvedURL
	}

	// Case 3: Hostname or container name - resolve via DNS
	log.Infof("Resolving hostname via DNS: %s", hostname)
	addrs, err := net.LookupIP(hostname)
	if err != nil {
		log.Warnf("DNS lookup failed for '%s': %v, using hostname as-is", hostname, err)
		// Return original URL even if DNS fails - it might still work
		resolvedURL := fmt.Sprintf("%s://%s:%s", scheme, hostname, port)
		return resolvedURL
	}

	if len(addrs) == 0 {
		log.Warnf("No IP addresses found for hostname '%s', using hostname as-is", hostname)
		resolvedURL := fmt.Sprintf("%s://%s:%s", scheme, hostname, port)
		return resolvedURL
	}

	// Use the first resolved IP address
	resolvedIP := addrs[0].String()
	resolvedURL := fmt.Sprintf("%s://%s:%s", scheme, resolvedIP, port)
	log.Infof("Resolved '%s' to %s", hostname, resolvedURL)
	return resolvedURL
}
