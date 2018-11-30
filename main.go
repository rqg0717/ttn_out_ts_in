package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"github.com/TheThingsNetwork/go-app-sdk"
	ttnlog "github.com/TheThingsNetwork/go-utils/log"
	"github.com/TheThingsNetwork/go-utils/log/apex"
	"github.com/TheThingsNetwork/ttn/core/types"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

const (
	sdkClientName string = "ttn_mqtt_in_out" // client name
	VERSION       string = "0.0.1"           // The version of the application
	NODE2         string = "node2"
	ThingSpeakAPI       string = "api.thingspeak.com"
)

type ThingSpeakChannel struct {
	tls       bool
	key       string
	fields    map[int]string
	lat       string
	long      string
	elevation string
	path      string
	url       string
}

func main() {
	var msg string

	log := apex.Stdout() // We use a cli logger at Stdout
	log.MustParseLevel("debug")
	ttnlog.Set(log) // Set the logger as default for TTN

	/*for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		fmt.Println(pair[0])
	}*/
	// We get the application ID and application access key from OS environment
	appID := os.Getenv("TTN_APP_ID")
	appAccessKey := os.Getenv("TTN_APP_ACCESS_KEY")

	msg = fmt.Sprintf("appID is %s.", appID)
	log.Info(msg)

	// Create a new SDK configuration for the public community network
	config := ttnsdk.NewCommunityConfig(sdkClientName)
	config.ClientVersion = VERSION

	// If you connect to a private network that does not use trusted certificates on the Discovery Server
	// (from Let's Encrypt for example), you have to manually trust the certificates. If you use the public community
	// network, you can just delete the next code block.
	if caCert := os.Getenv("TTN_CA_CERT"); caCert != "" {
		config.TLSConfig = new(tls.Config)
		certBytes, err := ioutil.ReadFile(caCert)
		if err != nil {
			msg = fmt.Sprintf("%s could not read CA certificate file.", sdkClientName)
			log.WithError(err).Fatal(msg)
		}
		config.TLSConfig.RootCAs = x509.NewCertPool()
		if ok := config.TLSConfig.RootCAs.AppendCertsFromPEM(certBytes); !ok {
			msg = fmt.Sprintf("%s could not read CA certificates.", sdkClientName)
			log.Fatal(msg)
		}
	}

	// Create a new SDK client for the application
	client := config.NewClient(appID, appAccessKey)

	// Make sure the client is closed before the function returns
	defer client.Close()

	// Manage devices for the application.
	devices, err := client.ManageDevices()
	if err != nil {
		msg = fmt.Sprintf("%s could not get device manager.", sdkClientName)
		log.WithError(err).Fatal(msg)
	}

	// List the first 10 devices
	deviceList, err := devices.List(10, 0)
	if err != nil {
		msg = fmt.Sprintf("%s could not get devices.", sdkClientName)
		log.WithError(err).Fatal(msg)
	}

	msg = fmt.Sprintf("%s get TTN devices.", sdkClientName)
	log.Info(msg)

	for _, device := range deviceList {
		fmt.Printf("- %s \n", device.DevID)
	}

	// Start Publish/Subscribe client (MQTT)
	pubsub, err := client.PubSub()
	if err != nil {
		msg = fmt.Sprintf("%s could not get application pub/sub.", sdkClientName)
		log.WithError(err).Fatal(msg)
	}

	// Make sure the pubsub client is closed before the function returns
	defer pubsub.Close()

	// Get a publish/subscribe client scoped to node 2
	myDevicePubSub := pubsub.Device(NODE2)

	// Make sure the pubsub client for this device is closed before the function returns
	defer myDevicePubSub.Close()

	// Subscribe to uplink messes
	uplink, err := myDevicePubSub.SubscribeUplink()
	if err != nil {
		msg = fmt.Sprintf("%s could not subscribe to uplink messages.", sdkClientName)
		log.WithError(err).Fatal(msg)
	}

	log.Debug("After this point, the program won't show anything until we receive an uplink message from the device.")
	var ch ThingSpeakChannel
	ch.key = "AJSKASJAKJSAKSJ"// shoud read from env
	ch.fields = make(map[int]string)

	for message := range uplink {
		hexPayload := hex.EncodeToString(message.PayloadRaw)
		msg = fmt.Sprintf("%s received uplink.", sdkClientName)
		log.WithField("data", hexPayload).Info(msg)

		//post values to ThingSpeak
		ch.fields[1] = "18.1"
		ch.fields[2] = "40.8"

		ch.path = "update"
		ch.url = fmt.Sprintf("https://%s/%s?api_key=%s", ThingSpeakAPI, ch.path, ch.key)
		for i, v:= range ch.fields {
			ch.url += fmt.Sprintf("&field%d=%s", i, v)
		}

		log.Debug(fmt.Sprintf("URL: %s\n", ch.url))

		var httpClient = &http.Client{
			Timeout: time.Second * 10,
		}
		request, err := http.NewRequest("POST", ch.url, nil)
		if err != nil {
			msg = fmt.Sprintf("%s could not post values to ThingSpeak.", sdkClientName)
			log.WithError(err).Fatal(msg)
		}
		resp, err := httpClient.Do(request)
		resp.Body.Close()

		break // normally you wouldn't do this
	}
	fmt.Println("Unsubscribe from uplink")
	// Unsubscribe from uplink
	err = myDevicePubSub.UnsubscribeUplink()
	if err != nil {
		msg = fmt.Sprintf("%s could not unsubscribe from uplink.", sdkClientName)
		log.WithError(err).Fatal(msg)
	}

	defer client.Close()
	defer pubsub.Close()
	defer myDevicePubSub.Close()
	// Publish downlink message
	err = myDevicePubSub.Publish(&types.DownlinkMessage{
		AppID:      appID, // can be left out, the SDK will fill this
		DevID:      NODE2, // can be left out, the SDK will fill this
		PayloadRaw: []byte{0xaa, 0xbb, 0xcc, 0xdd},
		FPort:      10,
		Schedule:   types.ScheduleLast, // allowed values: "replace" (default), "first", "last"
		Confirmed:  false,              // can be left out, default is false
	})
	if err != nil {
		msg = fmt.Sprintf("%s could not schedule downlink message.", sdkClientName)
		log.WithError(err).Fatal(msg)
	}
}
