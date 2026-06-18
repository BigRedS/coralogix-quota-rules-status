package blockedstatus

import (
	"fmt"
	"sort"
	"strings"
)

// regionHosts maps the short region name a user passes to the API host for that
// Coralogix region. These come straight from the `servers:` list in the
// Coralogix OpenAPI spec.
var regionHosts = map[string]string{
	"eu1": "api.coralogix.com",
	"eu2": "api.eu2.coralogix.com",
	"us1": "api.coralogix.us",
	"us2": "api.cx498.coralogix.com",
	"ap1": "api.coralogix.in",    // India
	"ap2": "api.coralogixsg.com", // Singapore
	"ap3": "api.ap3.coralogix.com",
}

// HostForRegion returns the API host for a region name, or an error listing the
// valid names if it isn't one we know.
func HostForRegion(region string) (string, error) {
	host, ok := regionHosts[strings.ToLower(region)]
	if !ok {
		return "", fmt.Errorf("unknown region %q (valid: eu1, eu2, us1, us2, ap1, ap2, ap3)", region)
	}
	return host, nil
}

// RegionChoice is a region code and its API host, for the web UI dropdown.
type RegionChoice struct {
	Code string `json:"code"`
	Host string `json:"host"`
}

// SortedRegions returns the regions as dropdown entries, sorted by code.
func SortedRegions() []RegionChoice {
	out := make([]RegionChoice, 0, len(regionHosts))
	for code, host := range regionHosts {
		out = append(out, RegionChoice{Code: code, Host: host})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}
