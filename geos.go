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

import "github.com/paulsmith/gogeos/geos"

func join(geometries []*geos.Geometry) ([]*geos.Geometry, error) {
	var result []*geos.Geometry
	if len(geometries) == 0 {
		return result, nil
	}
	var (
		err         error
		count       int
		newGeometry *geos.Geometry
	)

	// Union the geometries together
	newGeometry, err = geos.NewCollection(geos.GEOMETRYCOLLECTION, geometries...)

	// Merge the linestrings together when possible (when their endpoints match)
	newGeometry, err = newGeometry.LineMerge()
	if err != nil {
		return result, err
	}

	// Pull the resulting geometries out of the MultiLineString and
	// turn them to an array
	count, err = newGeometry.NGeometry()
	if err != nil {
		return result, err
	}
	for inx := 0; inx < count; inx++ {
		var geometry *geos.Geometry
		geometry, err = newGeometry.Geometry(inx)
		if err != nil {
			return result, err
		}
		result = append(result, geometry)
	}
	return result, nil
}
