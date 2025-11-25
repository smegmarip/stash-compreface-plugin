package config

// PluginConfig holds plugin settings from Stash
type PluginConfig struct {
	ComprefaceURL                  string
	RecognitionAPIKey              string
	DetectionAPIKey                string
	VerificationAPIKey             string
	VisionServiceURL               string
	QualityServiceURL              string
	StashHostURL                   string
	CooldownSeconds                int
	MaxBatchSize                   int
	MinSimilarity                  float64
	MinFaceSize                    int
	MinConfidenceScore        float64 // Minimum confidence score for face detection
	MinQualityScore           float64 // Minimum composite quality for subject creation (0=use component gates)
	MinProcessingQualityScore float64 // Minimum composite quality for recognition (0=use component gates)
	EnhanceQualityScoreTrigger     float64 // Quality score threshold to trigger enhancement
	ScannedTagName                 string
	MatchedTagName                 string
	PartialTagName                 string
	CompleteTagName                string
	SyncedTagName                  string
}
