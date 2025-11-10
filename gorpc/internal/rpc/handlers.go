package rpc

import (
	"fmt"

	"github.com/stashapp/stash/pkg/plugin/common"
	"github.com/stashapp/stash/pkg/plugin/common/log"
	"github.com/stashapp/stash/pkg/plugin/util"

	"github.com/smegmarip/stash-compreface-plugin/internal/compreface"
	"github.com/smegmarip/stash-compreface-plugin/internal/config"
	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
)

// Run handles RPC task execution
func (s *Service) Run(input common.PluginInput, output *common.PluginOutput) error {
	// Initialize GraphQL client and tag cache
	s.serverConnection = input.ServerConnection
	s.graphqlClient = util.NewClient(input.ServerConnection)
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
	var outputStr string = "Unknown mode"

	switch mode {
	case "synchronizePerformers":
		log.Info("Starting performer synchronization")
		err = s.synchronizePerformers()
		outputStr = "Performer synchronization completed"

	case "recognizeImagesHQ":
		log.Info("Starting high-quality image recognition")
		err = s.recognizeImages(false) // lowQuality=false
		outputStr = "High-quality image recognition completed"

	case "recognizeImagesLQ":
		log.Info("Starting low-quality image recognition")
		err = s.recognizeImages(true) // lowQuality=true
		outputStr = "Low-quality image recognition completed"

	case "identifyImagesAll":
		log.Info("Starting image identification (all)")
		err = s.identifyImages(false) // newOnly=false
		outputStr = "Image identification completed"

	case "identifyImagesNew":
		log.Info("Starting image identification (new only)")
		err = s.identifyImages(true) // newOnly=true
		outputStr = "New image identification completed"

	case "resetUnmatched":
		log.Info("Resetting unmatched images")
		err = s.resetUnmatchedImages()
		outputStr = "Unmatched images reset"

	case "recognizeScenes":
		log.Info("Starting scene recognition")
		err = s.recognizeScenes(false) // useSprites=false
		outputStr = "Scene recognition completed"

	case "recognizeSceneSprites":
		log.Info("Starting scene sprite recognition")
		err = s.recognizeScenes(true) // useSprites=true
		outputStr = "Scene sprite recognition completed"

	case "identifyImage":
		imageID := input.Args.String("imageId")
		createPerformer := input.Args.Bool("createPerformer")
		log.Infof("Identifying image: %s (createPerformer=%v)", imageID, createPerformer)
		err = s.identifyImage(imageID, createPerformer, nil)
		outputStr = "Image identification completed"

	case "createPerformerFromImage":
		imageID := input.Args.String("imageId")
		faceIndex := input.Args.Int("faceIndex")
		log.Infof("Creating performer from image: %s (faceIndex=%d)", imageID, faceIndex)
		err = s.identifyImage(imageID, true, &faceIndex)
		outputStr = "Performer created from image"

	case "identifyGallery":
		galleryID := input.Args.String("galleryId")
		createPerformer := input.Args.Bool("createPerformer")
		log.Infof("Identifying gallery: %s (createPerformer=%v)", galleryID, createPerformer)
		err = s.identifyGallery(galleryID, createPerformer)
		outputStr = "Gallery identification completed"

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
