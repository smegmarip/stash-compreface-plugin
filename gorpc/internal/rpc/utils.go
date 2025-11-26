package rpc

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/stashapp/stash/pkg/plugin/common/log"
)

// NormalizeHost normalizes localhost IP addresses in the given URL to the configured Stash host URL.
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
