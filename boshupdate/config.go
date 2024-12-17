package boshupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// GenericReleaseConfig -
type GenericReleaseConfig struct {
	Owner  string     `yaml:"owner"`
	Repo   string     `yaml:"repo"`
	Types  []string   `yaml:"types"`
	Format *Formatter `yaml:"format"`
}

func (c *GenericReleaseConfig) validate(name string) error {
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
func (c *GenericReleaseConfig) HasType(name string) bool {
	for _, t := range c.Types {
		if t == name {
			return true
		}
	}
	return false
}

// ManifestReleaseConfig -
type ManifestReleaseConfig struct {
	GenericReleaseConfig `yaml:",inline"`
	Manifest             string   `yaml:"manifest"`
	Ops                  []string `yaml:"ops"`
	Vars                 []string `yaml:"vars"`
	Matchers             []string `yaml:"matchers"`
}

func (c *ManifestReleaseConfig) Match(name string) bool {
	for _, m := range c.Matchers {
		re := regexp.MustCompile(m)
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

func (c *ManifestReleaseConfig) validate(name string) error {
	if err := c.GenericReleaseConfig.validate(name); err != nil {
		return err
	}
	if len(c.Matchers) == 0 {
		c.Matchers = append(c.Matchers, name+"(-.*)?")
	}
	for _, m := range c.Matchers {
		if _, err := regexp.Compile(m); err != nil {
			return fmt.Errorf("invalid match regexp '%s'", m)
		}
	}
	// if 0 == len(c.Manifest) {
	// 	return fmt.Errorf("missing mandatory manifest")
	// }
	return nil
}

// GithubConfig -
type GithubConfig struct {
	UpdateInterval   string                            `yaml:"update_interval"`
	Token            string                            `yaml:"token"`
	ManifestReleases map[string]*ManifestReleaseConfig `yaml:"manifest_releases"`
	GenericReleases  map[string]*GenericReleaseConfig  `yaml:"generic_releases"`
}

func (c *GithubConfig) validate() error {
	for name, data := range c.ManifestReleases {
		if err := data.validate(name); err != nil {
			return fmt.Errorf("invalid manifest release '%s', %s", name, err)
		}
	}
	for name, data := range c.GenericReleases {
		if err := data.validate(name); err != nil {
			return fmt.Errorf("invalid generic release '%s', %s", name, err)
		}
	}
	if 0 == len(c.Token) {
		return fmt.Errorf("missing mandatory github token")
	}
	_, err := time.ParseDuration(c.UpdateInterval)
	if err != nil {
		return fmt.Errorf("invalid duration format for update_interval")
	}

	return nil
}

// LogConfig -
type LogConfig struct {
	JSON  bool   `yaml:"json"`
	Level string `yaml:"level"`
}

// Config -
type Config struct {
	Log    LogConfig    `yaml:"log"`
	Bosh   BoshConfig   `yaml:"bosh"`
	Github GithubConfig `yaml:"github"`
}

// Validate - Validate configuration object
func (c *Config) Validate() error {
	if err := c.Github.validate(); err != nil {
		return fmt.Errorf("invalid github configuration: %s", err)
	}
	if err := c.Bosh.validate(); err != nil {
		return fmt.Errorf("invalid bosh configuration: %s", err)
	}
	return nil
}

// NewConfig - Creates and validates config from given reader
func NewConfig(file io.Reader) *Config {
	content, err := io.ReadAll(file)
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
