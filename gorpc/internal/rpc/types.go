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
