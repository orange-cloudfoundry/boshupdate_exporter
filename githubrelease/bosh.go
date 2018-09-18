package githubrelease

import (
	"fmt"
	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/cloudfoundry/bosh-cli/uaa"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	"os"
)

// BoshConfig -
type BoshConfig struct {
	URL          string `json:"url" yaml:"url"`
	LogLevel     string `json:"log_level" yaml:"log_level"`
	CaCert       string `json:"ca_cert" yaml:"ca_cert"`
	Username     string `json:"username" yaml:"username"`
	Password     string `json:"password" yaml:"password"`
	ClientID     string `json:"client_id" yaml:"client_id"`
	ClientSecret string `json:"client_secret" yaml:"client_secret"`
	Proxy        string `json:"proxy" yaml:"proxy"`
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
