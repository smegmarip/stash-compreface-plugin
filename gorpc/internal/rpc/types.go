package rpc

import (
	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/plugin/common"

	"github.com/smegmarip/stash-compreface-plugin/internal/compreface"
	"github.com/smegmarip/stash-compreface-plugin/internal/config"
	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
)

// Service is the main RPC service struct
type Service struct {
	stopping         bool
	serverConnection common.StashServerConnection
	graphqlClient    *graphql.Client
	config           *config.PluginConfig
	tagCache         *stash.TagCache
	comprefaceClient *compreface.Client
}

type PerformerData struct {
	Age    int     `json:"age"`
	Name   string  `json:"name"`
	ID     *string `json:"id,omitempty"`
	Gender string  `json:"gender"`
}

// FaceIdentity represents a recognized face identity
type FaceIdentity struct {
	ImageID     string               `json:"image_id"`
	BoundingBox *compreface.BoundingBox `json:"bounding_box,omitempty"`
	Performer   PerformerData        `json:"performer"`
	Confidence  *float64             `json:"confidence"`
}

// Response envelope for IdentifyImage RPC
type IdentifyImageResponse struct {
	Result *[]FaceIdentity `json:"result"`
}

// FaceQualityResult contains quality assessment outcome for CompreFace compatibility
type FaceQualityResult struct {
	Acceptable bool
	Reason     string
	Composite  float64
	Size       float64
	Pose       float64
	Occlusion  float64
	Sharpness  float64
}
