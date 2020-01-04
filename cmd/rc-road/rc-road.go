package main

import (
	"flag"
	"github.com/cyrilix/robocar-base/cli"
	"github.com/cyrilix/robocar-road/part"
	"log"
	"os"
)

const (
	DefaultClientId    = "robocar-road"
	DefaultHorizon = 20
)

func main() {
	var mqttBroker, username, password, clientId string
	var cameraTopic, roadTopic string
	var horizon int

	err := cli.SetIntDefaultValueFromEnv(&horizon, "HORIZON", DefaultHorizon)
	if err != nil {
		log.Printf("unable to parse horizon value arg: %v", err)
	}

	mqttQos := cli.InitIntFlag("MQTT_QOS", 0)
	_, mqttRetain := os.LookupEnv("MQTT_RETAIN")

	cli.InitMqttFlags(DefaultClientId, &mqttBroker, &username, &password, &clientId, &mqttQos, &mqttRetain)

	flag.StringVar(&roadTopic, "mqtt-topic-road", os.Getenv("MQTT_TOPIC_ROAD"), "Mqtt topic to publish road detection result, use MQTT_TOPIC_ROAD if args not set")
	flag.StringVar(&cameraTopic, "mqtt-topic-camera", os.Getenv("MQTT_TOPIC_CAMERA"), "Mqtt topic that contains camera frame values, use MQTT_TOPIC_CAMERA if args not set")
	flag.IntVar(&horizon, "horizon", horizon, "Limit horizon in pixels from top, use HORIZON if args not set")

	flag.Parse()
	if len(os.Args) <= 1 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	client, err := cli.Connect(mqttBroker, username, password, clientId)
	if err != nil {
		log.Fatalf("unable to connect to mqtt bus: %v", err)
	}
	defer client.Disconnect(50)

	p := part.NewRoadPart(client, horizon, cameraTopic, roadTopic)
	defer p.Stop()

	cli.HandleExit(p)

	err = p.Start()
	if err != nil {
		log.Fatalf("unable to start service: %v", err)
	}
}
