# bf-analyze

## Overview
A toolkit for analyzing the results of BeachFront shoreline detection.

### Building
`bf-analyze` is written in Go and depends on go-geos which requires GEOS, a C/C++ library.

1. https://github.com/paulsmith/gogeos#installation
1. `go build`

### What it Does
`./bf-analyze [detected] [baseline]` writes output of the detection to standard output.

#### Qualitative Analysis
The output is GeoJSON with a `Detection` property on each feature.

* `Detected` means the detected shoreline is in the baseline. 
In this case the geometry will be a GeometryCollection consisting of the detected geometry followed by the baseline geometry.
* `Not Detected` means a feature in the baseline was not detected.
* `New Detection` means a feature in the detected file does not have a corresponding entry in the baseline.

#### Quantitative Analysis
The quantitative analysis determines the amount of positive/negative space in a scene.
It constructs a MultiPolygon from the linework and then measures the area of each component polygon.
Area is measured twice - boundary area and total area (inner rings are not counted as part of total area.
From there we can output the sum of the positive and negative space in the scene. 
For now this output is logged; a better way to do this is TBD.

  
