package githubrelease

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/common/log"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

// type Formatter
type Formatter struct {
	Match   string `json:"match" yaml:"match"`
	Replace string `json:"replace" yaml:"replace"`
}

// GithubReleaseConfig -
type GithubReleaseConfig struct {
	Owner  string     `json:"owner" yaml:"owner"`
	Repo   string     `json:"repo"  yaml:"repo"`
	Types  []string   `json:"types" yaml:"types"`
	Format *Formatter `json:"format" yaml:"format"`
}

// Validate -
func (c *GithubReleaseConfig) Validate() error {
	if 0 == len(c.Owner) {
		return fmt.Errorf("missing mandatory owner")
	}
	if 0 == len(c.Repo) {
		return fmt.Errorf("missing mandatory repo")
	}
	if len(c.Types) == 0 {
		c.Types = []string{"release"}
	}
	if c.Format == nil {
		c.Format = &Formatter{
			Match:   "v([0-9.]+)",
			Replace: "${1}",
		}
	}

	if _, err := regexp.Compile(c.Format.Match); err != nil {
		return fmt.Errorf("invalid supplied regexp '%s' : %s", c.Format.Match, err)
	}

	for _, val := range c.Types {
		switch strings.ToLower(val) {
		case "release":
			return nil
		case "pre_release":
			return nil
		case "draft_release":
			return nil
		case "tag":
			return nil
		default:
			return fmt.Errorf("invalid release type '%s'", val)
		}
	}
	return nil
}

// HasType -
func (c *GithubReleaseConfig) HasType(name string) bool {
	for _, t := range c.Types {
		if t == name {
			return true
		}
	}
	return false
}

// BoshDeploymentConfig -
type BoshDeploymentConfig struct {
	GithubReleaseConfig `yaml:",inline"`
	Manifest            string   `json:"manifest" yaml:"manifest"`
	Ops                 []string `json:"ops" yaml:"ops"`
	Vars                []string `json:"vars" yaml:"vars"`
}

// Validate -
func (c *BoshDeploymentConfig) Validate() error {
	if err := c.GithubReleaseConfig.Validate(); err != nil {
		return err
	}
	if 0 == len(c.Manifest) {
		return fmt.Errorf("missing mandatory manifest")
	}
	return nil
}

// ReleaseData -
type ReleaseData struct {
	HasError bool   `json:"has-error" yaml:"has-error"`
	LastRef  string `json:"last-ref" yaml:"last-ref"`
	Version  string `json:"version" yaml:"version"`
}

// GithubReleaseData -
type GithubReleaseData struct {
	GithubReleaseConfig `yaml:",inline"`
	ReleaseData         `yaml:",inline"`
	Name                string `json:"name" yaml:"name"`
}

// NewGithubReleaseData -
func NewGithubReleaseData(config GithubReleaseConfig, name string) GithubReleaseData {
	return GithubReleaseData{
		GithubReleaseConfig: config,
		Name:                name,
	}
}

// BoshDeploymentData -
type BoshDeploymentData struct {
	BoshDeploymentConfig `yaml:",inline"`
	ReleaseData          `yaml:",inline"`
	Deployment           string        `json:"deployment" yaml:"deployment"`
	Releases             []BoshRelease `json:"releases" yaml:"releases"`
}

// NewBoshDeploymentData -
func NewBoshDeploymentData(config BoshDeploymentConfig, name string) BoshDeploymentData {
	return BoshDeploymentData{
		BoshDeploymentConfig: config,
		Deployment:           name,
	}
}

// Config -
type Config struct {
	Log struct {
		JSON  bool   `json:"json"     yaml:"json"`
		Level string `json:"level"     yaml:"level"`
	} `json:"log"     yaml:"log"`

	GithubToken    string                           `json:"github-token"     yaml:"github-token"`
	BoshDeployment map[string]*BoshDeploymentConfig `json:"bosh-deployments" yaml:"bosh-deployment"`
	GithubRelease  map[string]*GithubReleaseConfig  `json:"github-releases"  yaml:"github-release"`
}

// Validate -
func (c *Config) Validate() error {
	for name, data := range c.BoshDeployment {
		if err := data.Validate(); err != nil {
			return fmt.Errorf("invalid bosh deployment '%s', %s", name, err)
		}
	}
	for name, data := range c.GithubRelease {
		if err := data.Validate(); err != nil {
			return fmt.Errorf("invalid github release '%s', %s", name, err)
		}
	}
	if 0 == len(c.GithubToken) {
		return fmt.Errorf("missing mandatory github token")
	}
	return nil
}

// NewConfig -
func NewConfig(file io.Reader) *Config {
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("unable to read configuration file : %s", err)
		os.Exit(1)
	}

	config := Config{}
	if err = yaml.Unmarshal(content, &config); err != nil {
		if err = json.Unmarshal(content, &config); err != nil {
			log.Fatalf("unable to read configuration json/xml file: %s", err)
			os.Exit(1)
		}
	}

	if err = config.Validate(); err != nil {
		log.Fatalf("invalid configuration, %s", err)
		os.Exit(1)
	}
	return &config
}
