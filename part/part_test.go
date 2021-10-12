package part

import (
	"fmt"
	"github.com/cyrilix/robocar-base/testtools"
	"github.com/cyrilix/robocar-protobuf/go/events"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"sync"
	"testing"
	"time"
)

func TestRoadPart_OnFrame(t *testing.T) {
	oldRegister := registerCallBacks
	oldPublish := publish
	defer func() {
		registerCallBacks = oldRegister
		publish = oldPublish
	}()

	registerCallBacks = func(_ *RoadPart) {}

	var muEventsPublished sync.Mutex
	eventsPublished := make(map[string][]byte)
	publish = func(client mqtt.Client, topic string, payload *[]byte) {
		muEventsPublished.Lock()
		defer muEventsPublished.Unlock()
		eventsPublished[topic] = *payload
	}

	cameraTopic := "topic/camera"
	roadTopic := "topic/road"

	rp := NewRoadPart(nil, 20, cameraTopic, roadTopic)
	go func() {
		if err := rp.Start(); err != nil {
			t.Errorf("unable to start roadPart: %v", err)
			t.FailNow()
		}
	}()

	cases := []struct {
		name            string
		msg             mqtt.Message
		expectedCntr    []*events.Point
		expectedEllipse events.Ellipse
	}{
		{
			name:            "image1",
			msg:             loadFrame(t, cameraTopic, "image"),
			expectedCntr:    []*events.Point{&events.Point{X: 0, Y: int32(45)}, &events.Point{X: 0, Y: 127}, &events.Point{X: 144, Y: 127}, &events.Point{X: 95, Y: 21}, &events.Point{X: 43, Y: 21}},
			expectedEllipse: events.Ellipse{Center: &events.Point{X: 71, Y: 87}, Width: 139, Height: 176, Angle: 92.66927, Confidence: 1.},
		},
	}

	for _, c := range cases {
		rp.OnFrame(nil, c.msg)

		time.Sleep(20 * time.Millisecond)

		var roadMsg events.RoadMessage
		err := proto.Unmarshal(eventsPublished[roadTopic], &roadMsg)
		if err != nil {
			t.Errorf("unable to unmarshal response, bad return type: %v", err)
			continue
		}

		if len(roadMsg.Contour) != len(c.expectedCntr) {
			t.Errorf("[%v] bad nb point in road contour: %v, wants %v", c.name, len(roadMsg.Contour), len(c.expectedCntr))
		}
		for idx, pt := range roadMsg.Contour {
			if pt.String() != c.expectedCntr[idx].String() {
				t.Errorf("[%v] bad point at position %v: %v, wants %v", c.name, idx, pt, c.expectedCntr[idx])
			}
		}
		if roadMsg.Ellipse.String() != c.expectedEllipse.String() {
			t.Errorf("[%v] bad ellipse: %v, wants %v", c.name, roadMsg.Ellipse, c.expectedEllipse)
		}
		frameRef := frameRefFromPayload(c.msg.Payload())
		if frameRef.String() != roadMsg.GetFrameRef().String() {
			t.Errorf("[%v] invalid frameRef: %v, wants %v", c.name, roadMsg.GetFrameRef(), frameRef)
		}
	}
}

func frameRefFromPayload(payload []byte) *events.FrameRef {
	var msg events.FrameMessage
	err := proto.Unmarshal(payload, &msg)
	if err != nil {
		log.Errorf("unable to unmarchal %T msg: %v", msg, err)
	}
	return msg.GetId()
}

func loadFrame(t *testing.T, topic string, name string) mqtt.Message {
	img, err := ioutil.ReadFile(fmt.Sprintf("testdata/%s.jpg", name))
	if err != nil {
		t.Fatalf("unable to load data test image: %v", err)
		return nil
	}
	now := time.Now()
	msg := events.FrameMessage{
		Id: &events.FrameRef{
			Name: name,
			Id:   name,
			CreatedAt: &timestamp.Timestamp{
				Seconds: now.Unix(),
				Nanos:   int32(now.Nanosecond()),
			},
		},
		Frame: img,
	}
	return testtools.NewFakeMessageFromProtobuf(topic, &msg)
}
