package part

import (
	"github.com/cyrilix/robocar-protobuf/go/events"
	"go.uber.org/zap"
	"gocv.io/x/gocv"
	"image"
	"image/color"
)

const FILLED = -1

type RoadDetector struct {
	kernelSize                               int
	morphoIterations                         int
	approxPolyEpsilonFactor                  float64
	previousBoundingBox                      *image.Rectangle
	previousRoad                             *[]image.Point
	thresholdLowerBound, thresholdUpperBound gocv.Mat
}

func (rd *RoadDetector) Close() error {
	var err error
	err = nil
	if err1 := rd.thresholdLowerBound.Close(); err1 != nil {
		zap.S().Errorf("unable to close thresholdLowerBound resource: %v", err1)
		err = err1
	}
	if err2 := rd.thresholdUpperBound.Close(); err2 != nil {
		zap.S().Errorf("unable to close thresholdUpperBound resource: %v", err2)
		err = err2
	}
	return err
}

func NewRoadDetector() *RoadDetector {

	return &RoadDetector{
		kernelSize:              4,
		morphoIterations:        3,
		approxPolyEpsilonFactor: 0.01,
		thresholdLowerBound:     gocv.NewMatFromScalar(gocv.NewScalar(120.0, 120.0, 120.0, 120.0), gocv.MatTypeCV8U),
		thresholdUpperBound:     gocv.NewMatFromScalar(gocv.NewScalar(250.0, 250.0, 250.0, 250.0), gocv.MatTypeCV8U),
	}
}

func (rd *RoadDetector) DetectRoadContour(imgGray *gocv.Mat, horizonRow int) *gocv.PointVector {

	kernel := gocv.NewMatWithSizeFromScalar(gocv.NewScalar(1, 1, 1, 1), rd.kernelSize, rd.kernelSize, gocv.MatTypeCV8U)

	img := imgGray.Clone()
	defer func() {
		if err := img.Close(); err != nil {
			zap.S().Warnf("unable to close mat resource: %v", err)
		}
	}()

	for i := rd.morphoIterations; i > 0; i-- {
		gocv.Dilate(img, &img, kernel)
	}
	for i := rd.morphoIterations; i > 0; i-- {
		gocv.Erode(img, &img, kernel)
	}
	gocv.Dilate(img, &img, kernel)

	gocv.Threshold(img, &img, 180, 255, gocv.ThresholdBinaryInv)

	// Draw black rectangle above horizon
	horizon := gocv.NewMatWithSize(1, 4, gocv.MatTypeCV32S)
	horizon.SetIntAt(0, 0, 0)                     // X1
	horizon.SetIntAt(0, 1, int32(horizonRow))     // Y1
	horizon.SetIntAt(0, 2, int32(imgGray.Cols())) // X2
	horizon.SetIntAt(0, 3, int32(horizonRow))     // Y2
	rectangle := image.Rect(0, 0, int(horizon.GetIntAt(0, 2)), int(horizon.GetIntAt(0, 3)))
	gocv.Rectangle(&img, rectangle, color.RGBA{0, 0, 0, 0}, FILLED)

	return rd.detectRoadContour(&img)
}

func (rd *RoadDetector) detectRoadContour(imgInversed *gocv.Mat) *gocv.PointVector {

	var (
		epsilon float64
		cntr    gocv.PointVector
	)

	ptsVec := gocv.FindContours(*imgInversed, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer ptsVec.Close()

	if ptsVec.Size() == 0 {
		emptyContours := gocv.NewPointVector()
		return &emptyContours
	} else if ptsVec.Size() == 1 {
		epsilon = rd.approxPolyEpsilonFactor * gocv.ArcLength(ptsVec.At(0), true)
		cntr = ptsVec.At(0)
	} else {
		// Search biggest contour
		peris := make([]float64, ptsVec.Size())
		maxArcIdx := 0
		maxArcValue := 0.
		//for i, c := range cntrs {
		for i := 0; i< ptsVec.Size(); i++ {
			c := ptsVec.At(i)
			peri := gocv.ArcLength(c, true)
			peris[i] = peri
			if peri > maxArcValue {
				maxArcValue = peri
				maxArcIdx = i
			}
			cntr = ptsVec.At(maxArcIdx)
		}
		epsilon = rd.approxPolyEpsilonFactor * peris[maxArcIdx]
	}
	approx := gocv.ApproxPolyDP(cntr, epsilon, true)
	return &approx
}

var EllipseNotFound = events.Ellipse{Confidence: 0.}

func (rd *RoadDetector) ComputeEllipsis(road *gocv.PointVector) *events.Ellipse {
	if road.Size() < 5 {
		return &EllipseNotFound
	}

	rotatedRect := gocv.FitEllipse(*road)

	trust := rd.computeTrustFromCenter(&rotatedRect.Center)
	zap.S().Debugf("Trust: %v", trust)

	return &events.Ellipse{
		Center: &events.Point{
			X: int32(rotatedRect.Center.X),
			Y: int32(rotatedRect.Center.Y),
		},
		Width:      int32(rotatedRect.Width),
		Height:     int32(rotatedRect.Height),
		Angle:      float32(rotatedRect.Angle),
		Confidence: rd.computeTrustFromCenter(&rotatedRect.Center),
	}
}

func (rd *RoadDetector) computeTrustFromCenter(ellipsisCenter *image.Point) float32 {
	safeMinX := 48
	safeMaxX := 115
	safeMinY := 69
	safeMaxY := 119

	if safeMinX <= ellipsisCenter.X && ellipsisCenter.X <= safeMaxX && safeMinY <= ellipsisCenter.Y && ellipsisCenter.Y <= safeMaxY {
		return 1.0
	}

	if safeMinX <= ellipsisCenter.X && ellipsisCenter.X <= safeMaxX {
		return rd.computeTrustOnAxis(safeMaxY, safeMinY, ellipsisCenter.Y)
	}

	if safeMinY <= ellipsisCenter.Y && ellipsisCenter.Y <= safeMaxY {
		return rd.computeTrustOnAxis(safeMaxX, safeMinX, ellipsisCenter.X)
	}

	return rd.computeTrustOnAxis(safeMaxY, safeMinY, ellipsisCenter.Y) * rd.computeTrustOnAxis(safeMaxX, safeMinX, ellipsisCenter.X)
}

func (rd *RoadDetector) computeTrustOnAxis(safeMax, safeMin, value int) float32 {
	trust := 1.
	if value > safeMax {
		trust = 1. / float64(value-safeMax)
	} else if value < safeMin {
		trust = 1. / float64(safeMin-value)
	}
	trust = trust * 10.
	if trust > 0.9 {
		trust = 0.9
	}
	if trust < 0. {
		trust = 0.
	}
	return float32(trust)

}
