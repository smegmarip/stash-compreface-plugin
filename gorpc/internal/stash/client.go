package stash

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"

	graphql "github.com/hasura/go-graphql-client"

	"github.com/stashapp/stash/pkg/plugin/common"
)

// sanitize removes null JSON properties from GraphQL request bodies.
func sanitize(req *http.Request) {
	if req.Method != http.MethodPost || req.Body == nil {
		return
	}

	if req.Header.Get("Content-Type") != "application/json" {
		return
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		// If something goes wrong, just restore the body as-is
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		return
	}
	req.Body.Close()

	var data interface{}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		// Not valid JSON, restore the body and continue
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		return
	}

	cleaned := filter(data)
	cleanedBytes, err := json.Marshal(cleaned)
	if err != nil {
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		return
	}

	req.Body = io.NopCloser(bytes.NewBuffer(cleanedBytes))
	req.ContentLength = int64(len(cleanedBytes))
}

// Recursive helper function to remove `nil` (null) values
func filter(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		cleaned := make(map[string]interface{})
		for k, v2 := range val {
			if v2 == nil {
				continue
			}
			cleaned[k] = filter(v2)
		}
		return cleaned
	case []interface{}:
		for i, v2 := range val {
			val[i] = filter(v2)
		}
		return val
	default:
		return val
	}
}

// TestClient creates a GraphQL client for Stash with request sanitization
func TestClient(url string, httpClient graphql.Doer, options ...graphql.ClientOption) *graphql.Client {
	client := graphql.NewClient(url, httpClient, options...)
	return client.WithRequestModifier(sanitize)
}

// Client creates a graphql Client connecting to the stash server using
// the provided server connection details and a request sanitization modifier.
func Client(provider common.StashServerConnection) *graphql.Client {
	portStr := strconv.Itoa(provider.Port)

	u, _ := url.Parse("http://" + provider.Host + ":" + portStr + "/graphql")
	u.Scheme = provider.Scheme

	cookieJar, _ := cookiejar.New(nil)

	cookie := provider.SessionCookie
	if cookie != nil {
		cookieJar.SetCookies(u, []*http.Cookie{
			cookie,
		})
	}

	httpClient := &http.Client{
		Jar: cookieJar,
	}

	client := graphql.NewClient(u.String(), httpClient)
	return client.WithRequestModifier(sanitize)
}
