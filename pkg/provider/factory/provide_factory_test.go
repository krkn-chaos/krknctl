package factory

import (
	"github.com/krkn-chaos/krknctl/internal/config"
	"github.com/krkn-chaos/krknctl/pkg/provider"
	"github.com/krkn-chaos/krknctl/pkg/provider/offline"
	"github.com/krkn-chaos/krknctl/pkg/provider/quay"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProviderFactory_NewInstance(t *testing.T) {
	typeScenarioQuay := &quay.ScenarioProvider{}
	typeScenarioOffline := &offline.ScenarioProvider{}
	conf, err := config.LoadConfig()
	assert.Nil(t, err)
	assert.NotNil(t, conf)

	factory := NewProviderFactory(&conf)
	assert.NotNil(t, factory)

	factoryQuay := factory.NewInstance(provider.Online)
	assert.NotNil(t, factoryQuay)
	assert.IsType(t, factoryQuay, typeScenarioQuay)

	factoryOffline := factory.NewInstance(provider.Offline)
	assert.NotNil(t, factoryOffline)
	assert.IsType(t, factoryOffline, typeScenarioOffline)

}