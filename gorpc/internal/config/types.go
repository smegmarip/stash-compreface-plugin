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
	MinSceneConfidenceScore        float64 // Minimum confidence score for creating new subjects from scenes
	MinSceneQualityScore           float64 // Minimum quality score for creating new subjects from scenes
	MinSceneProcessingQualityScore float64 // Minimum quality score for processing faces from scenes
	EnhanceQualityScoreTrigger     float64 // Quality score threshold to trigger enhancement
	ScannedTagName                 string
	MatchedTagName                 string
	PartialTagName                 string
	CompleteTagName                string
	SyncedTagName                  string
}
