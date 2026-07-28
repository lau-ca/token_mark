package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCompositePolicySnapshotUsesAdministratorPublicModel(t *testing.T) {
	snapshot, err := buildCompositePolicySnapshot([]model.CompositeGroup{
		{
			Id:                1,
			Name:              "image_stable",
			PublicModel:       "admin-image-model",
			Status:            1,
			UserSelectable:    true,
			GenerationEnabled: true,
			Routes: []model.CompositeGroupRoute{
				{Operation: model.CompositeOperationGeneration, RouteOrder: 2, PhysicalGroup: "codex_image", InternalModel: "gpt-image-2", Status: 1},
				{Operation: model.CompositeOperationGeneration, RouteOrder: 1, PhysicalGroup: "gpt_image_web", InternalModel: "gpt-image-2-w", Status: 1},
			},
		},
	})
	require.NoError(t, err)

	policy, ok := snapshot.Resolve("image_stable")
	require.True(t, ok)
	assert.Equal(t, "admin-image-model", policy.PublicModel)
	routes := policy.RoutesFor(model.CompositeOperationGeneration)
	require.Len(t, routes, 2)
	assert.Equal(t, "gpt-image-2-w", routes[0].InternalModel)
}

func TestValidateCompositeGroupStructure(t *testing.T) {
	tests := []struct {
		name    string
		group   model.CompositeGroup
		routes  []model.CompositeGroupRoute
		wantErr string
	}{
		{
			name:    "public model required",
			group:   model.CompositeGroup{Name: "image_stable", GenerationEnabled: true},
			routes:  []model.CompositeGroupRoute{{Operation: model.CompositeOperationGeneration, RouteOrder: 1, PhysicalGroup: "web", InternalModel: "a", Status: 1}},
			wantErr: "public model",
		},
		{
			name:    "enabled operation requires route",
			group:   model.CompositeGroup{Name: "image_stable", PublicModel: "public", EditEnabled: true},
			wantErr: "image_edit",
		},
		{
			name:  "duplicate order",
			group: model.CompositeGroup{Name: "image_stable", PublicModel: "public", GenerationEnabled: true},
			routes: []model.CompositeGroupRoute{
				{Operation: model.CompositeOperationGeneration, RouteOrder: 1, PhysicalGroup: "web", InternalModel: "a", Status: 1},
				{Operation: model.CompositeOperationGeneration, RouteOrder: 1, PhysicalGroup: "codex", InternalModel: "b", Status: 1},
			},
			wantErr: "duplicate",
		},
		{
			name:    "retry count bounded",
			group:   model.CompositeGroup{Name: "image_stable", PublicModel: "public", GenerationEnabled: true},
			routes:  []model.CompositeGroupRoute{{Operation: model.CompositeOperationGeneration, RouteOrder: 1, PhysicalGroup: "web", InternalModel: "a", RetryCount: MaxCompositeRouteRetryCount + 1, Status: 1}},
			wantErr: "retry count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCompositeGroupStructure(tt.group, tt.routes)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestShouldRetryCompositeStatus(t *testing.T) {
	assert.True(t, ShouldRetryCompositeStatus("429,500-503,505-599", 429))
	assert.True(t, ShouldRetryCompositeStatus("429,500-503,505-599", 500))
	assert.False(t, ShouldRetryCompositeStatus("429,500-503,505-599", 400))
	assert.False(t, ShouldRetryCompositeStatus("429,500-503,505-599", 504))
}
