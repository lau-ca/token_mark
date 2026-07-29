package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildModelCapabilityCatalogUsesPricingModelsAndExactMetadata(t *testing.T) {
	pricings := []model.Pricing{
		{
			ModelName:              "square-exact-model",
			SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAI},
		},
		{
			ModelName:              "square-rule-model",
			SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeGemini},
		},
	}
	metadata := map[string]*model.Model{
		"square-exact-model": {
			Id:        11,
			ModelName: "square-exact-model",
			NameRule:  model.NameRuleExact,
		},
		"square-rule-model": {
			Id:        12,
			ModelName: "square-",
			NameRule:  model.NameRulePrefix,
		},
		"metadata-only-model": {
			Id:        13,
			ModelName: "metadata-only-model",
			NameRule:  model.NameRuleExact,
		},
	}

	items := buildModelCapabilityCatalog(pricings, metadata)

	require.Len(t, items, 2)
	assert.Equal(t, "square-exact-model", items[0].ModelName)
	require.NotNil(t, items[0].Metadata)
	assert.Equal(t, 11, items[0].Metadata.Id)
	assert.Equal(t, "square-rule-model", items[1].ModelName)
	assert.Nil(t, items[1].Metadata)
}
