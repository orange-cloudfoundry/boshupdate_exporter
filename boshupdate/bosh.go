package boshupdate

import (
	"fmt"
	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/cloudfoundry/bosh-cli/uaa"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
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
		return fmt.Errorf("missing mandatory url")
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

func readCACert(CACertFile string, logger logger.Logger) (string, error) {
	if CACertFile != "" {
		fs := system.NewOsFileSystem(logger)
		CACertFileFullPath, err := fs.ExpandPath(CACertFile)
		if err != nil {
			return "", err
		}
		CACert, err := fs.ReadFileString(CACertFileFullPath)
		if err != nil {
			return "", err
		}
		return CACert, nil
	}
	return "", nil
}

// NewDirector -
func NewDirector(config BoshConfig) (director.Director, error) {
	if config.Proxy != "" {
		os.Setenv("BOSH_ALL_PROXY", config.Proxy)
	}

	level, err := logger.Levelify(config.LogLevel)
	if err != nil {
		return nil, err
	}

	logger := logger.NewLogger(level)
	directorConfig, err := director.NewConfigFromURL(config.URL)
	if err != nil {
		return nil, err
	}

	boshCACert, err := readCACert(config.CaCert, logger)
	if err != nil {
		return nil, err
	}
	directorConfig.CACert = boshCACert

	anonymousDirector, err := director.NewFactory(logger).New(directorConfig, nil, nil)
	if err != nil {
		return nil, err
	}

	boshInfo, err := anonymousDirector.Info()
	if err != nil {
		return nil, err
	}

	if boshInfo.Auth.Type != "uaa" {
		directorConfig.Client = config.Username
		directorConfig.ClientSecret = config.Password
	} else {
		uaaURL := boshInfo.Auth.Options["url"]
		uaaURLStr, ok := uaaURL.(string)
		if !ok {
			return nil, fmt.Errorf("Expected UAA URL '%s' to be a string", uaaURL)
		}
		uaaConfig, err := uaa.NewConfigFromURL(uaaURLStr)
		if err != nil {
			return nil, err
		}
		uaaConfig.CACert = boshCACert

		if config.ClientID != "" && config.ClientSecret != "" {
			uaaConfig.Client = config.ClientID
			uaaConfig.ClientSecret = config.ClientSecret
		} else {
			uaaConfig.Client = "bosh_cli"
		}

		uaaFactory := uaa.NewFactory(logger)
		uaaClient, err := uaaFactory.New(uaaConfig)
		if err != nil {
			return nil, err
		}

		if config.ClientID != "" && config.ClientSecret != "" {
			directorConfig.TokenFunc = uaa.NewClientTokenSession(uaaClient).TokenFunc
		} else {
			answers := []uaa.PromptAnswer{
				uaa.PromptAnswer{
					Key:   "username",
					Value: config.Username,
				},
				uaa.PromptAnswer{
					Key:   "password",
					Value: config.Password,
				},
			}
			accessToken, err := uaaClient.OwnerPasswordCredentialsGrant(answers)
			if err != nil {
				return nil, err
			}

			origToken := uaaClient.NewStaleAccessToken(accessToken.RefreshToken().Value())
			directorConfig.TokenFunc = uaa.NewAccessTokenSession(origToken).TokenFunc
		}
	}

	boshFactory := director.NewFactory(logger)
	boshClient, err := boshFactory.New(directorConfig, director.NewNoopTaskReporter(), director.NewNoopFileReporter())
	if err != nil {
		return nil, err
	}

	return boshClient, nil
}
