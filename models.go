package main

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
