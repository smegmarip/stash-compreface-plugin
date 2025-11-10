package quality

import (
	"fmt"
	"image"
	"strings"
)

// QualityTier represents the quality level of a detected face
type QualityTier string

const (
	TierExcellent  QualityTier = "excellent"  // Best quality - high confidence + frontal
	TierGood       QualityTier = "good"       // Good quality - high confidence or frontal
	TierAcceptable QualityTier = "acceptable" // Acceptable - medium confidence + frontal
	TierPoor       QualityTier = "poor"       // Poor quality - barely usable
	TierUnusable   QualityTier = "unusable"   // Unusable - too small, masked, or very low quality
)

// QualityMetrics contains the quality assessment of a face
type QualityMetrics struct {
	Tier       QualityTier
	Confidence float64
	Pose       string
	Size       image.Point
	IsMasked   bool
	Reasons    []string // Why this tier was assigned
}

// AcceptancePolicy defines filtering rules for different scenarios
type AcceptancePolicy struct {
	// Minimum acceptable tier for different scenarios
	MinTierForNewSubject      QualityTier
	MinTierForMatching        QualityTier
	MinTierForBatchProcessing QualityTier

	// Allow override for explicit user selections
	AlwaysAcceptExplicitIndex bool

	// Similarity thresholds for matching based on quality
	ExcellentSimilarityThreshold  float64
	GoodSimilarityThreshold       float64
	AcceptableSimilarityThreshold float64

	// Quality thresholds
	ExcellentConfidenceThreshold  float64
	GoodConfidenceThreshold       float64
	AcceptableConfidenceThreshold float64
	MinFaceSize                   int

	// Fuzzy boundary ranges (0.0 = disabled, 0.05 = ±0.05 fuzzy zone)
	ConfidenceFuzzyRange float64
}

// Predefined policies
var (
	PolicyStrict = AcceptancePolicy{
		MinTierForNewSubject:          TierGood,
		MinTierForMatching:            TierAcceptable,
		MinTierForBatchProcessing:     TierGood,
		AlwaysAcceptExplicitIndex:     true,
		ExcellentSimilarityThreshold:  0.95,
		GoodSimilarityThreshold:       0.89,
		AcceptableSimilarityThreshold: 0.80,
		ExcellentConfidenceThreshold:  2.5,
		GoodConfidenceThreshold:       2.0,
		AcceptableConfidenceThreshold: 1.0,
		MinFaceSize:                   64,
		ConfidenceFuzzyRange:          0.03, // ±0.03 fuzzy zone (tighter for strict)
	}

	PolicyBalanced = AcceptancePolicy{
		MinTierForNewSubject:          TierAcceptable,
		MinTierForMatching:            TierAcceptable,
		MinTierForBatchProcessing:     TierAcceptable,
		AlwaysAcceptExplicitIndex:     true,
		ExcellentSimilarityThreshold:  0.95,
		GoodSimilarityThreshold:       0.89,
		AcceptableSimilarityThreshold: 0.80,
		ExcellentConfidenceThreshold:  2.5,
		GoodConfidenceThreshold:       2.0,
		AcceptableConfidenceThreshold: 1.0,
		MinFaceSize:                   64,
		ConfidenceFuzzyRange:          0.05, // ±0.05 fuzzy zone (balanced tolerance)
	}

	PolicyPermissive = AcceptancePolicy{
		MinTierForNewSubject:          TierPoor,
		MinTierForMatching:            TierPoor,
		MinTierForBatchProcessing:     TierAcceptable,
		AlwaysAcceptExplicitIndex:     true,
		ExcellentSimilarityThreshold:  0.90,
		GoodSimilarityThreshold:       0.85,
		AcceptableSimilarityThreshold: 0.75,
		ExcellentConfidenceThreshold:  2.5,
		GoodConfidenceThreshold:       2.0,
		AcceptableConfidenceThreshold: 1.0,
		MinFaceSize:                   64,
		ConfidenceFuzzyRange:          0.10, // ±0.10 fuzzy zone (wider for permissive)
	}
)

// FilterDecision contains the result of a filtering decision
type FilterDecision struct {
	Accepted bool
	Metrics  QualityMetrics
	Reason   string
}

// FaceFilter provides face quality assessment and filtering
type FaceFilter struct {
	policy AcceptancePolicy
}

// NewFaceFilter creates a new face filter with the given policy
func NewFaceFilter(policy AcceptancePolicy) *FaceFilter {
	return &FaceFilter{
		policy: policy,
	}
}

// NewFaceFilterByName creates a filter using a named policy
func NewFaceFilterByName(policyName string) *FaceFilter {
	var policy AcceptancePolicy

	switch strings.ToLower(policyName) {
	case "strict":
		policy = PolicyStrict
	case "balanced", "":
		policy = PolicyBalanced
	case "permissive":
		policy = PolicyPermissive
	default:
		policy = PolicyBalanced
	}

	return &FaceFilter{policy: policy}
}

// AssessQuality evaluates the quality of a detected face and assigns a tier
func (ff *FaceFilter) AssessQuality(face FaceDetection) QualityMetrics {
	metrics := QualityMetrics{
		Confidence: face.Confidence.Score,
		Pose:       face.Confidence.Type,
		Size:       face.Size,
		IsMasked:   face.Masked,
		Reasons:    []string{},
	}

	// Size-based rejection (always unusable)
	minDim := min(face.Size.X, face.Size.Y)
	if minDim < ff.policy.MinFaceSize {
		metrics.Tier = TierUnusable
		metrics.Reasons = append(metrics.Reasons, fmt.Sprintf("too_small_%dpx", minDim))
		return metrics
	}

	// Mask-based rejection (always unusable)
	if face.Masked {
		metrics.Tier = TierUnusable
		metrics.Reasons = append(metrics.Reasons, "masked")
		return metrics
	}

	// Determine tier based on confidence + pose combination
	isFrontal := isFrontalPose(metrics.Pose)

	// Use fuzzy boundary logic for tier assignment
	ff.assessQualityWithFuzzy(face, &metrics, isFrontal)
	return metrics
}

// ShouldCreateSubject determines if a face should create a new subject
func (ff *FaceFilter) ShouldCreateSubject(face FaceDetection, faceIndex *int) FilterDecision {
	metrics := ff.AssessQuality(face)

	// Always accept explicitly requested face index (e.g., user clicked this face)
	if ff.policy.AlwaysAcceptExplicitIndex && faceIndex != nil && *faceIndex == 0 {
		return FilterDecision{
			Accepted: true,
			Metrics:  metrics,
			Reason:   "explicit_user_selection",
		}
	}

	// Check tier against policy
	accepted := isTierAcceptable(metrics.Tier, ff.policy.MinTierForNewSubject)

	var reason string
	if accepted {
		reason = fmt.Sprintf("tier_%s_meets_minimum_%s", metrics.Tier, ff.policy.MinTierForNewSubject)
	} else {
		reason = fmt.Sprintf("tier_%s_below_minimum_%s", metrics.Tier, ff.policy.MinTierForNewSubject)
	}

	return FilterDecision{
		Accepted: accepted,
		Metrics:  metrics,
		Reason:   reason,
	}
}

// ShouldMatchToSubject determines if a face should match an existing subject
func (ff *FaceFilter) ShouldMatchToSubject(face FaceDetection, similarity float64) FilterDecision {
	metrics := ff.AssessQuality(face)

	// Determine required similarity threshold based on quality tier
	var threshold float64
	switch metrics.Tier {
	case TierExcellent:
		threshold = ff.policy.ExcellentSimilarityThreshold
	case TierGood:
		threshold = ff.policy.GoodSimilarityThreshold
	case TierAcceptable, TierPoor:
		threshold = ff.policy.AcceptableSimilarityThreshold
	default:
		threshold = 1.0 // Unusable faces can't match
	}

	// Also check if tier is acceptable for matching
	tierAcceptable := isTierAcceptable(metrics.Tier, ff.policy.MinTierForMatching)
	accepted := tierAcceptable && similarity >= threshold

	var reason string
	if !tierAcceptable {
		reason = fmt.Sprintf("tier_%s_below_minimum_%s", metrics.Tier, ff.policy.MinTierForMatching)
	} else if similarity < threshold {
		reason = fmt.Sprintf("similarity_%.3f_below_threshold_%.3f", similarity, threshold)
	} else {
		reason = fmt.Sprintf("tier_%s_similarity_%.3f_accepted", metrics.Tier, similarity)
	}

	return FilterDecision{
		Accepted: accepted,
		Metrics:  metrics,
		Reason:   reason,
	}
}

// ShouldProcessInBatch determines if a face should be processed in batch mode
func (ff *FaceFilter) ShouldProcessInBatch(face FaceDetection) FilterDecision {
	metrics := ff.AssessQuality(face)
	accepted := isTierAcceptable(metrics.Tier, ff.policy.MinTierForBatchProcessing)

	var reason string
	if accepted {
		reason = fmt.Sprintf("tier_%s_meets_batch_minimum_%s", metrics.Tier, ff.policy.MinTierForBatchProcessing)
	} else {
		reason = fmt.Sprintf("tier_%s_below_batch_minimum_%s", metrics.Tier, ff.policy.MinTierForBatchProcessing)
	}

	return FilterDecision{
		Accepted: accepted,
		Metrics:  metrics,
		Reason:   reason,
	}
}

// isTierAcceptable checks if a tier meets the minimum requirement
func isTierAcceptable(actual, minimum QualityTier) bool {
	tierOrder := map[QualityTier]int{
		TierExcellent:  5,
		TierGood:       4,
		TierAcceptable: 3,
		TierPoor:       2,
		TierUnusable:   1,
	}
	return tierOrder[actual] >= tierOrder[minimum]
}

// GetPolicy returns the current acceptance policy
func (ff *FaceFilter) GetPolicy() AcceptancePolicy {
	return ff.policy
}

// SetPolicy updates the acceptance policy
func (ff *FaceFilter) SetPolicy(policy AcceptancePolicy) {
	ff.policy = policy
}

// isFrontalPose determines if a pose type should be considered frontal
// Treats "front", "front-rotate-left", and "front-rotate-right" as frontal
func isFrontalPose(pose string) bool {
	poseLower := strings.ToLower(pose)
	return poseLower == "front" || strings.HasPrefix(poseLower, "front-rotate")
}
