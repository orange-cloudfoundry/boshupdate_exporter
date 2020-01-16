package boshupdate

import (
	"fmt"
	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/cloudfoundry/bosh-cli/uaa"
	"github.com/cloudfoundry/bosh-utils/logger"
	"io/ioutil"
	"os"
	"regexp"
)

// BoshConfig -
type BoshConfig struct {
	URL          string   `yaml:"url"`
	LogLevel     string   `yaml:"log_level"`
	CaCert       string   `yaml:"ca_cert"`
	Username     string   `yaml:"username"`
	Password     string   `yaml:"password"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	Excludes     []string `yaml:"excludes"`
	Proxy        string   `yaml:"proxy"`
}

func (c *BoshConfig) validate() error {
	if len(c.URL) == 0 {
		c.URL = os.Getenv("BOSH_ENVIRONMENT")
		if len(c.URL) == 0 {
			return fmt.Errorf("missing mandatory url")
		}
	}

	if len(c.ClientID) == 0 {
		c.ClientID = os.Getenv("BOSH_CLIENT")
	}
	if len(c.ClientSecret) == 0 {
		c.ClientSecret = os.Getenv("BOSH_CLIENT_SECRET")
	}
	if len(c.CaCert) == 0 {
		c.CaCert = os.Getenv("BOSH_CA_CERT")
	} else {
		val, err := ioutil.ReadFile(c.CaCert)
		if err != nil {
			return fmt.Errorf("unable to read file at path %s", c.CaCert)
		}
		c.CaCert = string(val)
	}

	if len(c.Proxy) == 0 {
		c.Proxy = os.Getenv("BOSH_ALL_PROXY")
	}

	for _, f := range c.Excludes {
		if _, err := regexp.Compile(f); err != nil {
			return fmt.Errorf("invalid exclude filter regexp '%s'", f)
		}
	}
	return nil
}

// IsExcluded - Tells if name is matching one of configured exclude filters
func (c *BoshConfig) IsExcluded(name string) bool {
	for _, f := range c.Excludes {
		if regexp.MustCompile(f).MatchString(name) {
			return true
		}
	}
	return false
}

func buildLogger(config BoshConfig) (logger.Logger, error) {
	level, err := logger.Levelify(config.LogLevel)
	if err != nil {
		return nil, err
	}
	logger := logger.NewLogger(level)
	return logger, nil
}

func buildUAA(url string, config BoshConfig, logger logger.Logger) (uaa.UAA, error) {
	uaaConfig, err := uaa.NewConfigFromURL(url)
	if err != nil {
		return nil, err
	}
	uaaConfig.CACert = config.CaCert
	uaaConfig.Client = config.ClientID
	uaaConfig.ClientSecret = config.ClientSecret
	uaaFactory := uaa.NewFactory(logger)
	return uaaFactory.New(uaaConfig)
}

func getDirectorInfo(config BoshConfig, logger logger.Logger) (*director.Info, error) {
	directorConfig, err := director.NewConfigFromURL(config.URL)
	if err != nil {
		return nil, err
	}
	directorConfig.CACert = config.CaCert
	factory := director.NewFactory(logger)
	anonymousDirector, err := factory.New(directorConfig, nil, nil)
	if err != nil {
		return nil, err
	}
	info, err := anonymousDirector.Info()
	return &info, err
}

// NewDirector -
func NewDirector(config BoshConfig) (director.Director, error) {
	if config.Proxy != "" {
		os.Setenv("BOSH_ALL_PROXY", config.Proxy)
	}

	logger, err := buildLogger(config)
	if err != nil {
		return nil, err
	}

	infos, err := getDirectorInfo(config, logger)
	if err != nil {
		return nil, err
	}

	directorConfig, err := director.NewConfigFromURL(config.URL)
	directorConfig.CACert = config.CaCert
	if infos.Auth.Type != "uaa" {
		directorConfig.Client = config.Username
		directorConfig.ClientSecret = config.Password
	} else {
		uaaURL := infos.Auth.Options["url"]
		uaaURLStr, ok := uaaURL.(string)
		if !ok {
			return nil, fmt.Errorf("Expected UAA URL '%s' to be a string", uaaURL)
		}
		uaaCli, err := buildUAA(uaaURLStr, config, logger)
		if err != nil {
			return nil, err
		}
		directorConfig.TokenFunc = uaa.NewClientTokenSession(uaaCli).TokenFunc
	}

	factory := director.NewFactory(logger)
	return factory.New(directorConfig, director.NewNoopTaskReporter(), director.NewNoopFileReporter())
}
