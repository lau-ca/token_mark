package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildModelCapabilityCatalogMergesConfiguredAndRuntimeModels(t *testing.T) {
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
	metadata := []*model.Model{
		{
			Id:        11,
			ModelName: "square-exact-model",
			NameRule:  model.NameRuleExact,
			Endpoints: `{"image-generation":{"playground":{"capabilities":["image.generate"]}}}`,
		},
		{
			Id:        12,
			ModelName: "square-",
			NameRule:  model.NameRulePrefix,
		},
		{
			Id:        13,
			ModelName: "metadata-only-model",
			NameRule:  model.NameRuleExact,
			Endpoints: `{"openai":{"playground":{"capabilities":["chat"]}}}`,
		},
	}

	items := buildModelCapabilityCatalog(pricings, metadata)

	require.Len(t, items, 3)
	assert.Equal(t, "metadata-only-model", items[0].ModelName)
	assert.False(t, items[0].Available)
	require.NotNil(t, items[0].Metadata)
	assert.Equal(t, 13, items[0].Metadata.Id)
	assert.Equal(t, "square-exact-model", items[1].ModelName)
	require.NotNil(t, items[1].Metadata)
	assert.True(t, items[1].Available)
	assert.Equal(t, 11, items[1].Metadata.Id)
	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeImageGeneration,
	}, items[1].SupportedEndpointTypes)
	assert.Equal(t, "square-rule-model", items[2].ModelName)
	assert.True(t, items[2].Available)
	assert.Nil(t, items[2].Metadata)
}

func TestBuildModelCapabilityCatalogKeepsConfiguredUnavailableModels(t *testing.T) {
	metadata := []*model.Model{
		{
			Id:        21,
			ModelName: "saved-video-model",
			NameRule:  model.NameRuleExact,
			Endpoints: `{"openai-video":{"playground":{"capabilities":["video.text_to_video"]}}}`,
		},
	}

	items := buildModelCapabilityCatalog(nil, metadata)

	require.Len(t, items, 1)
	assert.Equal(t, "saved-video-model", items[0].ModelName)
	assert.False(t, items[0].Available)
	require.NotNil(t, items[0].Metadata)
	assert.Equal(t, metadata[0].Endpoints, items[0].Metadata.Endpoints)
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIVideo}, items[0].SupportedEndpointTypes)
}

func TestBuildModelCapabilityCatalogDoesNotAttachRuleMetadata(t *testing.T) {
	pricings := []model.Pricing{{
		ModelName:              "rule-derived-model",
		SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeGemini},
	}}
	metadata := []*model.Model{{
		Id:        31,
		ModelName: "rule-",
		NameRule:  model.NameRulePrefix,
	}}

	items := buildModelCapabilityCatalog(pricings, metadata)

	require.Len(t, items, 1)
	assert.Equal(t, "rule-derived-model", items[0].ModelName)
	assert.True(t, items[0].Available)
	assert.Nil(t, items[0].Metadata)
}
