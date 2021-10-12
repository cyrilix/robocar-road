package part

import (
	"github.com/cyrilix/robocar-base/service"
	"github.com/cyrilix/robocar-protobuf/go/events"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"gocv.io/x/gocv"
)

type RoadPart struct {
	client                 mqtt.Client
	frameChan              chan frameToProcess
	readyForNext           chan interface{}
	cancel                 chan interface{}
	roadDetector           *RoadDetector
	horizon                int
	cameraTopic, roadTopic string
}

func NewRoadPart(client mqtt.Client, horizon int, cameraTopic, roadTopic string) *RoadPart {
	return &RoadPart{
		client:       client,
		frameChan:    make(chan frameToProcess),
		cancel:       make(chan interface{}),
		roadDetector: NewRoadDetector(),
		horizon:      horizon,
		cameraTopic:  cameraTopic,
		roadTopic:    roadTopic,
	}
}

func (r *RoadPart) Start() error {
	registerCallBacks(r)

	var frame = frameToProcess{}
	defer func() {
		if err := frame.Close(); err != nil {
			log.Errorf("unable to close msg: %v", err)
		}
	}()

	for {
		select {
		case f := <-r.frameChan:
			log.Debug("new msg")
			oldFrame := frame
			frame = f
			if err := oldFrame.Close(); err != nil {
				log.Errorf("unable to close msg: %v", err)
			}
			log.Debug("process msg")
			go r.processFrame(&frame)
		case <-r.cancel:
			log.Infof("Stop service")
			return nil
		}
	}
}

var registerCallBacks = func(r *RoadPart) {
	err := service.RegisterCallback(r.client, r.cameraTopic, r.OnFrame)
	if err != nil {
		log.Panicf("unable to register callback to topic %v:%v", r.cameraTopic, err)
	}
}

func (r *RoadPart) Stop() {
	defer func() {
		if err := r.roadDetector.Close(); err != nil {
			log.Errorf("unable to close roadDetector: %v", err)
		}
	}()
	close(r.readyForNext)
	close(r.cancel)
	service.StopService("road", r.client, r.roadTopic)
}

func (r *RoadPart) OnFrame(_ mqtt.Client, msg mqtt.Message) {
	var frameMsg events.FrameMessage
	err := proto.Unmarshal(msg.Payload(), &frameMsg)
	if err != nil {
		log.Errorf("unable to unmarshal %T message: %v", frameMsg, err)
		return
	}

	img, err := gocv.IMDecode(frameMsg.GetFrame(), gocv.IMReadUnchanged)
	if err != nil {
		log.Errorf("unable to decode image: %v", err)
		return
	}
	frame := frameToProcess{
		ref: frameMsg.GetId(),
		Mat: img,
	}
	r.frameChan <- frame
}

type frameToProcess struct {
	ref *events.FrameRef
	gocv.Mat
}

func (r *RoadPart) processFrame(frame *frameToProcess) {
	img := frame.Mat
	imgGray := gocv.NewMatWithSize(img.Rows(), img.Cols(), gocv.MatTypeCV8UC1)
	defer func() {
		if err := imgGray.Close(); err != nil {
			log.Warnf("unable to close Mat resource: %v", err)
		}
	}()
	gocv.CvtColor(img, &imgGray, gocv.ColorRGBToGray)

	road := r.roadDetector.DetectRoadContour(&imgGray, r.horizon)
	defer road.Close()

	ellipse := r.roadDetector.ComputeEllipsis(road)

	cntr := make([]*events.Point, 0, road.Size())
	for i:=0;i< road.Size(); i++ {
		pt := road.At(i)
		cntr = append(cntr, &events.Point{X: int32(pt.X), Y: int32(pt.Y)})
	}

	msg := events.RoadMessage{
		Contour:  cntr,
		Ellipse:  ellipse,
		FrameRef: frame.ref,
	}

	payload, err := proto.Marshal(&msg)
	if err != nil {
		log.Errorf("unable to marshal %T to protobuf: %err", msg, err)
		return
	}
	publish(r.client, r.roadTopic, &payload)
}

var publish = func(client mqtt.Client, topic string, payload *[]byte) {
	client.Publish(topic, 0, false, *payload)
}
