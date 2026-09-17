package main

import (
	"game/shared"
	"sort"
)

type RenderableObject struct {
	Obj          shared.GameObject
	ShadowLength float32
	SortKey      int32
}

func CalculateShadowLength(z uint32) float32 {
	normalized := float32(z) / 100.0
	if normalized > 1.0 {
		normalized = 1.0
	} else if normalized < 0.0 {
		normalized = 0.0
	}
	return normalized * 16.0
}

func SortObjectsForRendering(objects []shared.GameObject) []RenderableObject {
	var renderables []RenderableObject
	
	for _, obj := range objects {
		if obj.Z >= 11 && obj.Z <= 50 {
			shadowLength := CalculateShadowLength(obj.Z)
			zContribution := int32(obj.Z) / 2
			sortKey := int32(obj.Y) - zContribution
			
			renderables = append(renderables, RenderableObject{
				Obj:          obj,
				ShadowLength: shadowLength,
				SortKey:      sortKey,
			})
		}
	}
	
	sort.Slice(renderables, func(i, j int) bool {
		return renderables[i].SortKey < renderables[j].SortKey
	})
	
	return renderables
}
