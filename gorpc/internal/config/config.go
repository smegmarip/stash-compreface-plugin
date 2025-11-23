package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/stashapp/stash/pkg/plugin/common"
	"github.com/stashapp/stash/pkg/plugin/common/log"
)

// Load loads and validates plugin configuration from Stash settings
func Load(input common.PluginInput) (*PluginConfig, error) {
	config := &PluginConfig{
		// Default values
		CooldownSeconds:                10,
		MaxBatchSize:                   20,
		MinSimilarity:                  0.81,
		MinFaceSize:                    64,
		MinSceneConfidenceScore:        0.7,
		MinSceneQualityScore:           0.65,
		MinSceneProcessingQualityScore: 0.2,
		EnhanceQualityScoreTrigger:     0.5,
		ScannedTagName:                 "Compreface Scanned",
		MatchedTagName:                 "Compreface Matched",
		PartialTagName:                 "Compreface Partial",
		CompleteTagName:                "Compreface Complete",
		SyncedTagName:                  "Compreface Synced",
	}

	// Fetch plugin configuration from Stash
	pluginConfig, err := getPluginConfiguration(input.ServerConnection)
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
		if val := getFloatSetting(pluginConfig, "minSceneConfidenceScore"); val > 0 {
			config.MinSceneConfidenceScore = val
		}
		if val := getFloatSetting(pluginConfig, "minSceneQualityScore"); val > 0 {
			config.MinSceneQualityScore = val
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
		if val := getStringSetting(pluginConfig, "stashHostUrl"); val != "" {
			config.StashHostURL = val
		}
	}

	// Resolve Compreface URL with auto-detection
	config.ComprefaceURL = resolveServiceURL(config.ComprefaceURL, "compreface", "8000")

	// Resolve Vision Service URL with auto-detection (optional service)
	if config.VisionServiceURL != "" {
		config.VisionServiceURL = resolveServiceURL(config.VisionServiceURL, "vision-api", "5010")
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

	if config.StashHostURL != "" {
		config.StashHostURL = resolveServiceURL(config.StashHostURL, "host.docker.internal", "9999")
		log.Infof("Stash Host URL configured at: %s", config.StashHostURL)
	} else {
		log.Info("Stash Host URL set to server connection (auto-detection)")
		config.StashHostURL = fmt.Sprintf("%s://%s:%d", input.ServerConnection.Scheme, input.ServerConnection.Host, input.ServerConnection.Port)
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

// getPluginConfiguration fetches plugin configuration from Stash via GraphQL HTTP request
func getPluginConfiguration(serverConnection common.StashServerConnection) (map[string]interface{}, error) {
	// Build Stash GraphQL URL
	stashURL := fmt.Sprintf("%s://%s:%d/graphql", serverConnection.Scheme, serverConnection.Host, serverConnection.Port)

	// Query Stash for plugin configuration
	query := `{ "query": "{ configuration { plugins } }" }`

	req, err := http.NewRequestWithContext(context.Background(), "POST", stashURL, strings.NewReader(query))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if serverConnection.SessionCookie != nil {
		req.AddCookie(serverConnection.SessionCookie)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query Stash: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GraphQL request failed with status %d", resp.StatusCode)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract plugin configuration
	data, ok := result["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing data")
	}

	configuration, ok := data["configuration"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing configuration")
	}

	plugins, ok := configuration["plugins"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing plugins")
	}

	// Get configuration for "compreface-rpc" plugin
	pluginConfig, ok := plugins["compreface-rpc"].(map[string]interface{})
	if !ok {
		log.Debug("No configuration found for compreface-rpc plugin, using defaults")
		return make(map[string]interface{}), nil
	}

	log.Debugf("Loaded plugin configuration with %d settings", len(pluginConfig))
	return pluginConfig, nil
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
	val, ok := config[key]
	if !ok || val == nil {
		return 0
	}
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case int32:
		return int(v)
	case uint:
		return int(v)
	case uint64:
		return int(v)
	case uint32:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	case bool:
		if v {
			return 1
		}
		return 0
	case string:
		// Try integer parse
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
		// Try parsing as float
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return int(f)
		}
		// Try parsing "true"/"false"
		if b, err := strconv.ParseBool(v); err == nil {
			if b {
				return 1
			}
			return 0
		}
		// No valid string conversion
		return 0
	default:
		// Unsupported type
		return 0
	}
}

// getFloatSetting retrieves a float setting from plugin config
func getFloatSetting(config map[string]interface{}, key string) float64 {
	val, ok := config[key]
	if !ok || val == nil {
		return 0.0
	}

	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case uint:
		return float64(v)
	case uint64:
		return float64(v)
	case uint32:
		return float64(v)
	case float32:
		return float64(v)
	case bool:
		if v {
			return 1.0
		}
		return 0.0
	case string:
		// Try parsing as float
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
		// Try parsing "true"/"false"
		if b, err := strconv.ParseBool(v); err == nil {
			if b {
				return 1.0
			}
			return 0.0
		}
		// No valid string conversion
		return 0.0
	default:
		// Unsupported type
		return 0.0
	}
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

	// Case 1b: host.docker.internal - use as-is (Docker special hostname, no DNS resolution)
	if hostname == "host.docker.internal" {
		resolvedURL := fmt.Sprintf("%s://%s:%s", scheme, hostname, port)
		log.Infof("Using Docker host gateway URL: %s", resolvedURL)
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
