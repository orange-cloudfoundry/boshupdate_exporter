package githubrelease

// GithubRef -
type GithubRef struct {
	Ref  string
	Unix int64
}

// BoshRelease -
type BoshRelease struct {
	Name    string `yaml:"name" json:"name"`
	URL     string `yaml:"url" json:"url"`
	Version string `yaml:"version" json:"version"`
}

// BoshManifest -
type BoshManifest struct {
	Releases []BoshRelease `yaml:"releases"`
}
