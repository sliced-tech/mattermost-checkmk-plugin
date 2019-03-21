package main

import (
	"fmt"
	"github.com/pkg/errors"
)

type configuration struct {
	CmkBaseUrl  string
	CmkUsername string
	CmkSecret   string

	disabled bool
}

func (c *configuration) Clone() *configuration {
	return &configuration{
		CmkBaseUrl:  c.CmkBaseUrl,
		CmkUsername: c.CmkUsername,
		CmkSecret:   c.CmkSecret,
	}
}

func (p *CheckMKPlugin) OnConfigurationChange() error {
	p.API.LogDebug("ON CONFIGURATION CHANGE")
	var configuration = new(configuration)

	if loadConfigErr := p.API.LoadPluginConfiguration(configuration); loadConfigErr != nil {
		return errors.Wrap(loadConfigErr, "failed to load plugin configuration")
	}
	p.setConfiguration(configuration)

	p.API.LogDebug("CONFIGURATION")
	str := fmt.Sprintf("%#v", p.getConfiguration())
	p.API.LogDebug(str)
	return nil
}

func (p *CheckMKPlugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &configuration{}
	}

	return p.configuration
}

func (p *CheckMKPlugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = configuration
}

// setEnabled wraps setConfiguration to configure if the plugin is enabled.
func (p *CheckMKPlugin) setEnabled(enabled bool) {
	var configuration = p.getConfiguration().Clone()
	configuration.disabled = !enabled

	p.setConfiguration(configuration)
}
