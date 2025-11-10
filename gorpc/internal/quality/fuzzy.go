package quality

// assessQualityWithFuzzy evaluates quality with fuzzy boundary logic
// This method provides smoother tier transitions for faces near confidence thresholds
func (ff *FaceFilter) assessQualityWithFuzzy(face FaceDetection, metrics *QualityMetrics, isFrontal bool) {
	conf := metrics.Confidence
	fuzzy := ff.policy.ConfidenceFuzzyRange

	// If fuzzy range is 0, use hard cutoffs (backward compatible)
	if fuzzy == 0 {
		ff.assessQualityHard(metrics, isFrontal)
		return
	}

	// Fuzzy boundary helper
	inFuzzyZone := func(value, threshold, fuzzyRange float64) (below, in, above bool) {
		below = value < (threshold - fuzzyRange)
		above = value >= (threshold + fuzzyRange)
		in = !below && !above
		return
	}

	// Check Excellent threshold with fuzzy logic
	_, excellentFuzzy, excellentAbove := inFuzzyZone(
		conf,
		ff.policy.ExcellentConfidenceThreshold,
		fuzzy,
	)

	if excellentAbove && isFrontal {
		// Definitely excellent
		metrics.Tier = TierExcellent
		metrics.Reasons = append(metrics.Reasons, "high_conf_frontal")
		return
	}

	if excellentFuzzy && isFrontal {
		// Fuzzy zone - use secondary factors
		if len(face.Landmarks) >= 5 && face.Size.X >= 100 {
			// Has landmarks and good size - upgrade to excellent
			metrics.Tier = TierExcellent
			metrics.Reasons = append(metrics.Reasons, "fuzzy_excellent_upgraded")
		} else {
			// Degrade to good
			metrics.Tier = TierGood
			metrics.Reasons = append(metrics.Reasons, "fuzzy_excellent_degraded")
		}
		return
	}

	// Check Good threshold with fuzzy logic
	_, goodFuzzy, goodAbove := inFuzzyZone(
		conf,
		ff.policy.GoodConfidenceThreshold,
		fuzzy,
	)

	if goodAbove {
		if isFrontal {
			metrics.Tier = TierGood
			metrics.Reasons = append(metrics.Reasons, "good_conf_frontal")
		} else {
			metrics.Tier = TierAcceptable
			metrics.Reasons = append(metrics.Reasons, "good_conf_profile")
		}
		return
	}

	if goodFuzzy {
		// Fuzzy zone around Good threshold
		if isFrontal && len(face.Landmarks) >= 5 {
			// Frontal with landmarks - upgrade to good
			metrics.Tier = TierGood
			metrics.Reasons = append(metrics.Reasons, "fuzzy_good_upgraded")
		} else if isFrontal {
			// Frontal but no landmarks - acceptable
			metrics.Tier = TierAcceptable
			metrics.Reasons = append(metrics.Reasons, "fuzzy_good_degraded_frontal")
		} else {
			// Profile - definitely acceptable
			metrics.Tier = TierAcceptable
			metrics.Reasons = append(metrics.Reasons, "fuzzy_good_profile")
		}
		return
	}

	// Check Acceptable threshold with fuzzy logic
	acceptableBelow, acceptableFuzzy, acceptableAbove := inFuzzyZone(
		conf,
		ff.policy.AcceptableConfidenceThreshold,
		fuzzy,
	)

	if acceptableAbove && isFrontal {
		metrics.Tier = TierAcceptable
		metrics.Reasons = append(metrics.Reasons, "medium_conf_frontal")
		return
	}

	if acceptableFuzzy && isFrontal {
		// Fuzzy zone - degrade to poor but still usable
		metrics.Tier = TierPoor
		metrics.Reasons = append(metrics.Reasons, "fuzzy_acceptable_degraded")
		return
	}

	// Below acceptable threshold - use standard logic
	if acceptableBelow {
		// Poor tier logic (from original)
		if conf >= ff.policy.AcceptableConfidenceThreshold ||
			(conf >= 0.5 && isFrontal) {
			metrics.Tier = TierPoor
			metrics.Reasons = append(metrics.Reasons, "low_quality")
			return
		}

		// Negative confidence but frontal (rotated faces)
		if conf < 0 && isFrontal {
			metrics.Tier = TierPoor
			metrics.Reasons = append(metrics.Reasons, "negative_conf_frontal")
			return
		}

		// Unusable
		metrics.Tier = TierUnusable
		metrics.Reasons = append(metrics.Reasons, "very_low_conf")
		return
	}
}

// assessQualityHard uses hard confidence cutoffs (no fuzzy logic)
// This is the original behavior when ConfidenceFuzzyRange = 0
func (ff *FaceFilter) assessQualityHard(metrics *QualityMetrics, isFrontal bool) {
	conf := metrics.Confidence

	// Excellent: High confidence + frontal
	if conf >= ff.policy.ExcellentConfidenceThreshold && isFrontal {
		metrics.Tier = TierExcellent
		metrics.Reasons = append(metrics.Reasons, "high_conf_frontal")
		return
	}

	// Good: High confidence (any pose) OR medium-high confidence + frontal
	if conf >= ff.policy.GoodConfidenceThreshold {
		if isFrontal {
			metrics.Tier = TierGood
			metrics.Reasons = append(metrics.Reasons, "good_conf_frontal")
		} else {
			metrics.Tier = TierAcceptable
			metrics.Reasons = append(metrics.Reasons, "good_conf_profile")
		}
		return
	}

	// Acceptable: Medium confidence + frontal
	if conf >= ff.policy.AcceptableConfidenceThreshold && isFrontal {
		metrics.Tier = TierAcceptable
		metrics.Reasons = append(metrics.Reasons, "medium_conf_frontal")
		return
	}

	// Poor: Medium confidence + profile, or low confidence + frontal
	if conf >= ff.policy.AcceptableConfidenceThreshold ||
		(conf >= 0.5 && isFrontal) {
		metrics.Tier = TierPoor
		metrics.Reasons = append(metrics.Reasons, "low_quality")
		return
	}

	// Poor (special case): Negative confidence but frontal pose
	if conf < 0 && isFrontal {
		metrics.Tier = TierPoor
		metrics.Reasons = append(metrics.Reasons, "negative_conf_frontal")
		return
	}

	// Unusable: Everything else
	metrics.Tier = TierUnusable
	metrics.Reasons = append(metrics.Reasons, "very_low_conf")
}
