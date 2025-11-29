package rpc

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/stashapp/stash/pkg/plugin/common"
	"github.com/stashapp/stash/pkg/plugin/common/log"

	"github.com/smegmarip/stash-compreface-plugin/internal/compreface"
	"github.com/smegmarip/stash-compreface-plugin/internal/config"
	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
)

// Run handles RPC task execution
func (s *Service) Run(input common.PluginInput, output *common.PluginOutput) error {
	// Initialize GraphQL client and tag cache
	s.serverConnection = input.ServerConnection
	s.graphqlClient = stash.Client(input.ServerConnection)
	s.tagCache = stash.NewTagCache()

	// Load plugin configuration
	cfg, err := config.Load(input)
	if err != nil {
		return s.errorOutput(output, fmt.Errorf("failed to load config: %w", err))
	}
	s.config = cfg

	// Initialize Compreface client
	s.comprefaceClient = compreface.NewClient(
		cfg.ComprefaceURL,
		cfg.RecognitionAPIKey,
		cfg.DetectionAPIKey,
		cfg.VerificationAPIKey,
		cfg.MinSimilarity,
	)

	log.Infof("Compreface plugin started - mode: %s", input.Args.String("mode"))
	log.Debugf("Configuration: URL=%s, BatchSize=%d, Cooldown=%ds",
		cfg.ComprefaceURL, cfg.MaxBatchSize, cfg.CooldownSeconds)

	mode := input.Args.String("mode")

	// Parse limit parameter (Stash sends integers as float64 in JSON)
	limit := 0
	argsMap := input.Args.ToMap()
	if limitVal, ok := argsMap["limit"]; ok {
		switch v := limitVal.(type) {
		case float64:
			limit = int(v)
		case int:
			limit = v
		case string:
			// Try parsing string as int
			if val, err := strconv.Atoi(v); err == nil {
				limit = val
			}
		}
	}
	log.Debugf("Mode: %s, Limit: %d", mode, limit)

	var outputStr string = "Unknown mode"

	switch mode {
	case "synchronizePerformers":
		log.Infof("Starting performer synchronization (limit=%d)", limit)
		err = s.synchronizePerformers(limit)
		outputStr = "Performer synchronization completed"

	case "recognizeImages":
		log.Infof("Starting image recognition (limit=%d)", limit)
		err = s.recognizeImages(limit)
		outputStr = "Image recognition completed"

	case "identifyImagesAll":
		log.Infof("Starting image identification (all, limit=%d)", limit)
		err = s.identifyImages(false, limit) // newOnly=false
		outputStr = "Image identification completed"

	case "identifyImagesNew":
		log.Infof("Starting image identification (new only, limit=%d)", limit)
		err = s.identifyImages(true, limit) // newOnly=true
		outputStr = "New image identification completed"

	case "resetUnmatchedImages":
		log.Infof("Resetting unmatched images (limit=%d)", limit)
		err = s.resetUnmatchedImages(limit)
		outputStr = "Unmatched images reset"

	case "recognizeNewScenes":
		log.Infof("Starting scene recognition (limit=%d)", limit)
		err = s.recognizeScenes(false, false, limit) // useSprites=false scanPartial=false
		outputStr = "Scene recognition completed"

	case "recognizeAllScenes":
		log.Infof("Starting scene recognition (limit=%d)", limit)
		err = s.recognizeScenes(false, true, limit) // useSprites=false scanPartial=true
		outputStr = "Scene recognition completed"

	case "recognizeNewSceneSprites":
		log.Infof("Starting scene sprite recognition (limit=%d)", limit)
		err = s.recognizeScenes(true, false, limit) // useSprites=true scanPartial=false
		outputStr = "Scene sprite recognition completed"

	case "recognizeAllSceneSprites":
		log.Infof("Starting scene sprite recognition (limit=%d)", limit)
		err = s.recognizeScenes(true, true, limit) // useSprites=true scanPartial=true
		outputStr = "Scene sprite recognition completed"

	case "identifyImage":
		// Parse imageId (Stash sends integers as float64 in JSON)
		imageID := ""
		if imageVal, ok := argsMap["imageId"]; ok {
			switch v := imageVal.(type) {
			case float64:
				imageID = fmt.Sprintf("%.0f", v)
			case int:
				imageID = fmt.Sprintf("%d", v)
			case string:
				imageID = v
			}
		}
		var _res *[]FaceIdentity
		createPerformer := input.Args.Bool("createPerformer")
		associateExisting := input.Args.Bool("associateExisting")
		log.Infof("Identifying image: %s (createPerformer=%v associateExisting=%v)", imageID, createPerformer, associateExisting)
		_res, err = s.identifyImage(imageID, createPerformer, associateExisting, nil)
		response := IdentifyImageResponse{Result: _res}
		res, _err := json.Marshal(response)
		if _err == nil {
			log.Infof("identifyImage=%s", string(res))
		}
		outputStr = "Image identification completed"

	case "createPerformerFromImage":
		// Parse imageId (Stash sends integers as float64 in JSON)
		imageID := ""
		if imageVal, ok := argsMap["imageId"]; ok {
			switch v := imageVal.(type) {
			case float64:
				imageID = fmt.Sprintf("%.0f", v)
			case int:
				imageID = fmt.Sprintf("%d", v)
			case string:
				imageID = v
			}
		}
		faceIndex := 0
		if indexVal, ok := argsMap["faceIndex"]; ok {
			switch v := indexVal.(type) {
			case float64:
				faceIndex = int(v)
			case int:
				faceIndex = v
			case string:
				faceIndex, _ = strconv.Atoi(v)
			}
		}
		log.Infof("Creating performer from image: %s (faceIndex=%d)", imageID, faceIndex)
		// When creating a performer, always associate with the image
		_, err = s.identifyImage(imageID, true, true, &faceIndex)
		outputStr = "Performer created from image"

	case "identifyGallery":
		// Parse galleryId (Stash sends integers as float64 in JSON)
		galleryID := ""
		if galleryVal, ok := argsMap["galleryId"]; ok {
			switch v := galleryVal.(type) {
			case float64:
				galleryID = fmt.Sprintf("%.0f", v)
			case int:
				galleryID = fmt.Sprintf("%d", v)
			case string:
				galleryID = v
			}
		}
		createPerformer := input.Args.Bool("createPerformer")
		log.Infof("Identifying gallery: %s (createPerformer=%v, limit=%d)", galleryID, createPerformer, limit)
		err = s.identifyGallery(galleryID, createPerformer, limit)
		outputStr = "Gallery identification completed"

	case "resetUnmatchedScenes":
		log.Infof("Resetting unmatched scenes (limit=%d)", limit)
		err = s.resetUnmatchedScenes(limit)
		outputStr = "Unmatched scenes reset"

	default:
		err = fmt.Errorf("unknown mode: %s", mode)
	}

	if err != nil {
		return s.errorOutput(output, err)
	}

	*output = common.PluginOutput{
		Output: &outputStr,
	}

	return nil
}
