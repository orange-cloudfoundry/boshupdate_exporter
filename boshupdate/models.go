package boshupdate

import (
	"regexp"
)

// Formatter -
type Formatter struct {
	Match   string `yaml:"match"`
	Replace string `yaml:"replace"`
}

// Format - Format input string according to Match regexp and Replace directive
func (s *Formatter) Format(ref string) string {
	re := regexp.MustCompile(s.Match)
	return re.ReplaceAllString(ref, s.Replace)
}

// DoesMatch -
func (s *Formatter) DoesMatch(ref string) bool {
	re := regexp.MustCompile(s.Match)
	return re.MatchString(ref)
}

// GithubRef -
type GithubRef struct {
	Ref  string
	Time int64
}

// Version -
type Version struct {
	GitRef       string `yaml:"gitref"`
	Version      string `yaml:"version"`
	Time         int64  `yaml:"time"`
	ExpiredSince int64  `yaml:"expired_since"`
}

func NewVersion(gitref string, version string, timestamp int64) Version {
	return Version{
		GitRef:       gitref,
		Version:      version,
		Time:         timestamp,
		ExpiredSince: 0,
	}
}

// GetStatus -
func (r Version) GetStatus(latest Version) string {
	if r.Version == latest.Version {
		return "latest"
	}
	return "deprecated"
}

// BoshDeploymentData -
type BoshDeploymentData struct {
	Deployment   string        `yaml:"deployment"`
	ManifestName string        `yaml:"manifest"`
	Ref          string        `yaml:"current"`
	HasError     bool          `yaml:"has_error"`
	BoshReleases []BoshRelease `yaml:"bosh_releases"`
}

// GenericReleaseData -
type GenericReleaseData struct {
	GenericReleaseConfig `yaml:",inline"`
	HasError             bool      `yaml:"has-error"`
	Versions             []Version `yaml:"versions"`
	LatestVersion        Version   `yaml:"latest"`
	Name                 string    `yaml:"name"`
}

// NewGenericReleaseData -
func NewGenericReleaseData(config GenericReleaseConfig, name string) GenericReleaseData {
	return GenericReleaseData{
		GenericReleaseConfig: config,
		Name:                 name,
	}
}

// BoshRelease -
type BoshRelease struct {
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	Version string `yaml:"version"`
}

// ManifestReleaseData -
type ManifestReleaseData struct {
	ManifestReleaseConfig `yaml:",inline"`
	HasError              bool          `yaml:"has-error"`
	Name                  string        `yaml:"name"`
	Versions              []Version     `yaml:"versions"`
	LatestVersion         Version       `yaml:"latest"`
	BoshReleases          []BoshRelease `yaml:"bosh_releases"`
}

// NewManifestReleaseData -
func NewManifestReleaseData(config ManifestReleaseConfig, name string) ManifestReleaseData {
	return ManifestReleaseData{
		ManifestReleaseConfig: config,
		Name:                  name,
	}
}

// BoshManifest -
type BoshManifest struct {
	Releases []BoshRelease `yaml:"releases"`
}
