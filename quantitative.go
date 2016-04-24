/*
Copyright 2016, RadiantBlue Technologies, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"log"

	"github.com/paulsmith/gogeos/geos"
)

type polygonMetadata struct {
	boundaryArea, totalArea float64
	terminal                bool
	link                    *polygonMetadata
	index                   int
}

func quantitativeReview(scene Scene, envelope *geos.Geometry) error {
	var (
		err          error
		polygon      *geos.Geometry
		polygon2     *geos.Geometry
		mpolygon     *geos.Geometry
		boundary     *geos.Geometry
		geometries   *geos.Geometry
		count        int
		touches      bool
		positiveArea float64
		negativeArea float64
	)

	if geometries, err = scene.MultiLineString(); err != nil {
		return err
	}
	if mpolygon, err = mlsToMPoly(geometries); err != nil {
		return err
	}
	if count, err = mpolygon.NGeometry(); err != nil {
		return err
	}
	var polygonMetadatas = make([]polygonMetadata, count)

	for inx := 0; inx < count; inx++ {
		polygonMetadatas[inx].index = inx
		polygon, err = mpolygon.Geometry(inx)
		if err != nil {
			return err
		}
		// We need two areas for each component polygon
		// The total area (which considers holes)
		if polygonMetadatas[inx].totalArea, err = polygon.Area(); err != nil {
			return err
		}
		// The shell (boundary)
		if boundary, err = polygon.Shell(); err != nil {
			return err
		}
		if boundary, err = geos.PolygonFromGeom(boundary); err != nil {
			return err
		}
		if polygonMetadatas[inx].boundaryArea, err = boundary.Area(); err != nil {
			return err
		}

		// Construct an ordered acyclical graph of spaces,
		// with the first polygon being the terminal node
		if inx == 0 {
			polygonMetadatas[inx].terminal = true
		}
		// Iterate through all of the polygons
		for jnx := 1; jnx < count; jnx++ {
			// If a polygon is not already linked
			if (inx == jnx) || (polygonMetadatas[jnx].link != nil) {
				continue
			}
			if polygon2, err = mpolygon.Geometry(jnx); err != nil {
				return err
			}
			if touches, err = polygon2.Touches(polygon); err != nil {
				return err
			}
			// And it touches the current polygon, register the link
			if touches {
				polygonMetadatas[jnx].link = &(polygonMetadatas[inx])
			}
		}
	}
	for inx := 0; inx < count; inx++ {
		counter := 0
		// Count the steps to get from the current polygon to the terminal one
		// to determine its polarity
		for current := inx; !polygonMetadatas[current].terminal; {
			current = polygonMetadatas[current].link.index
			counter++
		}
		switch counter % 2 {
		case 0:
			positiveArea += polygonMetadatas[inx].totalArea
			negativeArea += polygonMetadatas[inx].boundaryArea - polygonMetadatas[inx].totalArea
		case 1:
			negativeArea += polygonMetadatas[inx].totalArea
			positiveArea += polygonMetadatas[inx].boundaryArea - polygonMetadatas[inx].totalArea
		}
	}
	log.Printf("+:%v -:%v Sum: %v Total:%v\n", positiveArea, negativeArea, positiveArea-negativeArea, positiveArea+negativeArea)
	return err
}
