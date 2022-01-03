package testtools

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type fakeMessage struct {
	qos     byte
	topic   string
	payload []byte
	acked   bool
}

func (f *fakeMessage) Duplicate() bool {
	return false
}

func (f *fakeMessage) Qos() byte {
	return f.qos
}

func (f *fakeMessage) Retained() bool {
	return false
}

func (f *fakeMessage) Topic() string {
	return f.topic
}

func (f *fakeMessage) MessageID() uint16 {
	return 1234
}

func (f *fakeMessage) Payload() []byte {
	return f.payload
}

func (f *fakeMessage) Ack() {
	f.acked = true
}

func NewFakeMessage(topic string, payload []byte) mqtt.Message {
	return &fakeMessage{
		qos:     0,
		topic:   topic,
		payload: payload,
		acked:   false,
	}
}

func NewFakeMessageFromProtobuf(topic string, msg proto.Message) mqtt.Message {
	payload, err := proto.Marshal(msg)
	if err != nil {
		zap.S().Errorf("unable to marshal protobuf message %T: %v", msg, err)
		return nil
	}
	return NewFakeMessage(topic, payload)
}
