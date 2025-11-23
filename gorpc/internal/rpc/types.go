package rpc

import (
	"net/url"
	"regexp"
	"strings"

	graphql "github.com/hasura/go-graphql-client"
	"github.com/stashapp/stash/pkg/plugin/common"

	"github.com/smegmarip/stash-compreface-plugin/internal/compreface"
	"github.com/smegmarip/stash-compreface-plugin/internal/config"
	"github.com/smegmarip/stash-compreface-plugin/internal/stash"
	"github.com/stashapp/stash/pkg/plugin/common/log"
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
	ImageID    string        `json:"image_id"`
	Image      *string       `json:"image"`
	Performer  PerformerData `json:"performer"`
	Confidence *float64      `json:"confidence"`
}

// Response envelope for IdentifyImage RPC
type IdentifyImageResponse struct {
	Result *[]FaceIdentity `json:"result"`
}

func (s *Service) NormalizeHost(urlStr string) string {
	log.Debugf("Normalizing URL host for: %s", urlStr)
	hostName := "0.0.0.0"
	config := s.config
	u, err := url.Parse(urlStr)
	if err != nil {
		log.Warnf("Failed to parse URL %s: %v", urlStr, err)
		return urlStr
	}
	log.Debugf("Parsed URL host: %s", u.Host)
	if strings.HasPrefix(u.Host, hostName) {
		log.Debugf("Detected localhost IP, normalizing to %s", config.StashHostURL)
		re := regexp.MustCompile(`http[s]?://` + regexp.QuoteMeta(hostName) + `(:\d+)?`)
		return re.ReplaceAllString(urlStr, config.StashHostURL)
	}
	return urlStr
}
