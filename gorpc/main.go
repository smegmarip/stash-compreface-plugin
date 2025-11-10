package main

import (
	"github.com/smegmarip/stash-compreface-plugin/internal/rpc"
	"github.com/stashapp/stash/pkg/plugin/common"
)

func main() {
	service := rpc.NewService()
	err := common.ServePlugin(service)
	if err != nil {
		panic(err)
	}
}
