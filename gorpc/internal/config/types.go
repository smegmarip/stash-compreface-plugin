package config

// PluginConfig holds plugin settings from Stash
type PluginConfig struct {
	ComprefaceURL      string
	RecognitionAPIKey  string
	DetectionAPIKey    string
	VerificationAPIKey string
	VisionServiceURL   string
	QualityServiceURL  string
	CooldownSeconds    int
	MaxBatchSize       int
	MinSimilarity      float64
	MinFaceSize        int
	ScannedTagName     string
	MatchedTagName     string
}
