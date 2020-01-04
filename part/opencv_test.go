package part

import (
	"fmt"
	"github.com/cyrilix/robocar-protobuf/go/events"
	log "github.com/sirupsen/logrus"
	"gocv.io/x/gocv"
	"image"
	"image/color"
	"testing"
)

func toGray(imgColor gocv.Mat) *gocv.Mat {
	imgGray := gocv.NewMatWithSize(imgColor.Rows(), imgColor.Cols(), gocv.MatTypeCV8UC1)
	gocv.CvtColor(imgColor, &imgGray, gocv.ColorRGBToGray)
	return &imgGray
}

func image1() *gocv.Mat {
	img := gocv.IMRead("testdata/image.jpg", gocv.IMReadColor)
	return &img
}

func image2() *gocv.Mat {
	img := gocv.IMRead("testdata/image2.jpg", gocv.IMReadColor)
	return &img
}

func image3() *gocv.Mat {
	img := gocv.IMRead("testdata/image3.jpg", gocv.IMReadColor)
	return &img
}

func image4() *gocv.Mat {
	img := gocv.IMRead("testdata/image4.jpg", gocv.IMReadColor)
	return &img
}

func image5() *gocv.Mat {
	img := gocv.IMRead("testdata/image5.jpg", gocv.IMReadColor)
	return &img
}

func TestRoadDetection_DetectRoadContour(t *testing.T) {
	rd := NewRoadDetector()

	img1 := image1()
	defer img1.Close()
	img2 := image2()
	defer img2.Close()
	img3 := image3()
	defer img3.Close()
	img4 := image4()
	defer img4.Close()
	img5 := image5()
	defer img5.Close()

	cases := []struct {
		name            string
		img             *gocv.Mat
		horizon         int
		expectedContour []image.Point
	}{
		{"image1", img1, 20,
			[]image.Point{image.Point{0, 45}, image.Point{0, 127}, image.Point{144, 127}, image.Point{95, 21}, image.Point{43, 21}},
		},
		{"image2", img2, 20,
			[]image.Point{{159,69}, {128,53}, {125,41}, {113,42}, {108,21}, {87,21}, {79,41}, {72,30}, {44,39}, {29,34}, {0,67}, {0,127}, {159,127}, {152,101},},
		},
		{"image3", img3, 20,
			[]image.Point{{97,21}, {59,127}, {159,127}, {159,36}, {138,21},},
		},
		{"image4", img4, 20,
			[]image.Point{{0,21}, {0,77}, {68,22}, {0,96}, {0,127}, {159,127}, {159,21},},
		},
		{"image5", img5, 20,
			[]image.Point{{159,32}, {100,36}, {29,60}, {0,79}, {0,127}, {159,127},},
		},
	}

	for _, c := range cases {
		imgGray := toGray(*c.img)
		contours := rd.DetectRoadContour(imgGray, c.horizon)
		imgGray.Close()

		log.Infof("[%v] contour: %v", c.name, *contours)
		if len(*contours) != len(c.expectedContour) {
			t.Errorf("[%v] bad contour size: %v point(s), wants %v", c.name, len(*contours), len(c.expectedContour))
		}
		for idx, pt := range c.expectedContour {
			if pt != (*contours)[idx] {
				t.Errorf("[%v] bad point: %v, wants %v", c.name, (*contours)[idx], pt)
			}
		}
		debugContour(*c.img, contours, fmt.Sprintf("/tmp/%v.jpg", c.name))
	}
}

func debugContour(img gocv.Mat, contour *[]image.Point, imgPath string) {
	imgColor := img.Clone()
	defer imgColor.Close()

	gocv.DrawContours(&imgColor, [][]image.Point{*contour,}, 0, color.RGBA{
		R: 0,
		G: 255,
		B: 0,
		A: 255,
	}, 1)
	gocv.IMWrite(imgPath, imgColor)
}

func TestRoadDetector_ComputeEllipsis(t *testing.T) {
	rd := NewRoadDetector()

	cases := []struct {
		name            string
		contour []image.Point
		expectedEllipse events.Ellipse
	}{
		{"image1",
			[]image.Point{image.Point{0, 45}, image.Point{0, 127}, image.Point{144, 127}, image.Point{95, 21}, image.Point{43, 21}},
			events.Ellipse{
				Center: &events.Point{
					X:                    71,
					Y:                    87,
				},
				Width:                139,
				Height:               176,
				Angle:                92.66927,
				Confidence:           1.,
			},
		},
		{"image2",
			[]image.Point{{159,69}, {128,53}, {125,41}, {113,42}, {108,21}, {87,21}, {79,41}, {72,30}, {44,39}, {29,34}, {0,67}, {0,127}, {159,127}, {152,101},},
			events.Ellipse{
				Center: &events.Point{
					X:                    77,
					Y:                    102,
				},
				Width:                152,
				Height:               168,
				Angle:                94.70433,
				Confidence:           1.,
			},
		},
		{"image3",
			[]image.Point{{97,21}, {59,127}, {159,127}, {159,36}, {138,21},},
			events.Ellipse{
				Center: &events.Point{
					X:                    112,
					Y:                    86,
				},
				Width:                122,
				Height:               140,
				Angle:                20.761106,
				Confidence:           1.,
			},
		},
		{"image4",
			[]image.Point{{0,21}, {0,77}, {68,22}, {0,96}, {0,127}, {159,127}, {159,21},},
			events.Ellipse{
				Center: &events.Point{
					X:                    86,
					Y:                    78,
				},
				Width:                154,
				Height:               199,
				Angle:                90.45744,
				Confidence:           1.,
			},
		},
		{"image5",
			[]image.Point{{159,32}, {100,36}, {29,60}, {0,79}, {0,127}, {159,127},},
			events.Ellipse{
				Center: &events.Point{
					X:                    109,
					Y:                    87,
				},
				Width:                103,
				Height:               247,
				Angle:                79.6229,
				Confidence:           1.0,
			},
		},
	}

	for _, c := range cases{
		ellipse := rd.ComputeEllipsis(&c.contour)
		if ellipse.String() != c.expectedEllipse.String(){
			t.Errorf("ComputeEllipsis(%v): %v, wants %v", c.name, ellipse.String(), c.expectedEllipse.String())
		}
	}
}
