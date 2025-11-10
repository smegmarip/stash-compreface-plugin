package quality

import (
	"fmt"
	"net"
	"net/url"
	"os"
)

// QualityMode defines the quality assessment backend to use
type QualityMode string

const (
	ModeGoInternal    QualityMode = "go-internal"    // Use go-face (dlib via CGO)
	ModePythonService QualityMode = "python-service" // Use Python Quality Service
	ModeAuto          QualityMode = "auto"           // Auto-select based on operation type
)

// QualityRouter routes quality assessment requests to appropriate backend
type QualityRouter struct {
	mode          QualityMode
	goDetector    *Detector
	pythonClient  *PythonClient
	filter        *FaceFilter
	modelsDir     string
	pythonURL     string
	minConfidence float64
}

// RouterConfig holds configuration for QualityRouter
type RouterConfig struct {
	Mode              QualityMode
	ModelsDir         string
	PythonServiceURL  string
	QualityPolicyName string
	MinConfidence     float64
}

// NewQualityRouter creates a new quality assessment router
func NewQualityRouter(config RouterConfig) (*QualityRouter, error) {
	router := &QualityRouter{
		mode:          config.Mode,
		modelsDir:     config.ModelsDir,
		pythonURL:     config.PythonServiceURL,
		minConfidence: config.MinConfidence,
	}

	// Create filter
	if config.QualityPolicyName == "" {
		config.QualityPolicyName = "balanced"
	}
	router.filter = NewFaceFilterByName(config.QualityPolicyName)

	// Initialize backends based on mode
	switch config.Mode {
	case ModeGoInternal:
		if err := router.initGoDetector(); err != nil {
			return nil, fmt.Errorf("failed to initialize go-face detector: %w", err)
		}

	case ModePythonService:
		if err := router.initPythonClient(); err != nil {
			return nil, fmt.Errorf("failed to initialize Python client: %w", err)
		}

	case ModeAuto:
		// Initialize both backends, with fallback
		if err := router.initGoDetector(); err != nil {
			// Go detector failed, try Python service
			if err := router.initPythonClient(); err != nil {
				return nil, fmt.Errorf("failed to initialize both backends: go-face: %v, python: %v", err, err)
			}
		} else {
			// Go detector succeeded, optionally init Python as fallback
			_ = router.initPythonClient() // Ignore error - fallback is optional
		}

	default:
		return nil, fmt.Errorf("invalid quality mode: %s", config.Mode)
	}

	return router, nil
}

// initGoDetector initializes the go-face detector
func (r *QualityRouter) initGoDetector() error {
	if r.modelsDir == "" {
		return fmt.Errorf("models directory not specified")
	}

	config := DetectorConfig{
		ModelsDir:     r.modelsDir,
		MinConfidence: r.minConfidence,
	}

	detector, err := NewDetector(config)
	if err != nil {
		return err
	}

	r.goDetector = detector
	return nil
}

// initPythonClient initializes the Python Quality Service client
func (r *QualityRouter) initPythonClient() error {
	// Resolve service URL
	serviceURL := resolvePythonServiceURL(r.pythonURL)

	client := NewPythonClient(serviceURL)

	// Health check
	if err := client.Health(); err != nil {
		return fmt.Errorf("python service health check failed: %w", err)
	}

	r.pythonClient = client
	return nil
}

// DetectFaces detects faces in an image using the configured backend
func (r *QualityRouter) DetectFaces(imagePath string, batchMode bool) ([]FaceDetection, error) {
	// Route based on mode and operation type
	backend := r.selectBackend(batchMode)

	switch backend {
	case ModeGoInternal:
		if r.goDetector == nil {
			return nil, fmt.Errorf("go-face detector not initialized")
		}
		return r.goDetector.DetectFile(imagePath)

	case ModePythonService:
		if r.pythonClient == nil {
			return nil, fmt.Errorf("python client not initialized")
		}
		// Read image file
		imageData, err := readImageFile(imagePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read image: %w", err)
		}
		return r.pythonClient.Detect(imageData)

	default:
		return nil, fmt.Errorf("no backend available")
	}
}

// selectBackend chooses the appropriate backend based on mode and operation type
func (r *QualityRouter) selectBackend(batchMode bool) QualityMode {
	switch r.mode {
	case ModeGoInternal:
		return ModeGoInternal

	case ModePythonService:
		return ModePythonService

	case ModeAuto:
		// Auto mode: use go-face for single operations, Python for batch
		if batchMode {
			// Batch mode: prefer Python service for comprehensive detection
			if r.pythonClient != nil {
				return ModePythonService
			}
			// Fallback to go-face if Python unavailable
			if r.goDetector != nil {
				return ModeGoInternal
			}
		} else {
			// Single operation: prefer go-face for speed and precision
			if r.goDetector != nil {
				return ModeGoInternal
			}
			// Fallback to Python if go-face unavailable
			if r.pythonClient != nil {
				return ModePythonService
			}
		}

	default:
		// Unknown mode - try available backends
		if r.goDetector != nil {
			return ModeGoInternal
		}
		if r.pythonClient != nil {
			return ModePythonService
		}
	}

	return "" // No backend available
}

// GetFilter returns the configured FaceFilter
func (r *QualityRouter) GetFilter() *FaceFilter {
	return r.filter
}

// Close releases resources
func (r *QualityRouter) Close() {
	if r.goDetector != nil {
		r.goDetector.Close()
	}
}

// resolvePythonServiceURL resolves the Python service URL with DNS lookup
func resolvePythonServiceURL(configuredURL string) string {
	const defaultContainerName = "stash-face-quality"
	const defaultPort = "6001"
	const defaultScheme = "http"

	// If no URL configured, use fallback
	if configuredURL == "" {
		configuredURL = fmt.Sprintf("%s://%s:%s", defaultScheme, defaultContainerName, defaultPort)
	}

	// Parse the URL
	parsedURL, err := url.Parse(configuredURL)
	if err != nil {
		return fmt.Sprintf("%s://localhost:%s", defaultScheme, defaultPort)
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
		return fmt.Sprintf("%s://%s:%s", scheme, hostname, port)
	}

	// Case 2: Already an IP address - use as-is
	if net.ParseIP(hostname) != nil {
		return fmt.Sprintf("%s://%s:%s", scheme, hostname, port)
	}

	// Case 3: Hostname or container name - resolve via DNS
	addrs, err := net.LookupIP(hostname)
	if err != nil || len(addrs) == 0 {
		// DNS lookup failed, use hostname as-is
		return fmt.Sprintf("%s://%s:%s", scheme, hostname, port)
	}

	// Use the first resolved IP address
	resolvedIP := addrs[0].String()
	return fmt.Sprintf("%s://%s:%s", scheme, resolvedIP, port)
}

// readImageFile reads an image file from disk
func readImageFile(imagePath string) ([]byte, error) {
	return os.ReadFile(imagePath)
}
