package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"github.com/tilinna/z85"
)

var cert tls.Certificate

func handler(writer http.ResponseWriter, request *http.Request) {
	components := strings.Split(request.URL.Path, "/")

	token := components[2]

	buffer := new(bytes.Buffer)
    buffer.ReadFrom(request.Body)
    body := buffer.Bytes()

	encodedBytes := make([]byte, z85.EncodedLen(len(body)))
	_, err := z85.Encode(encodedBytes, body)
	if err != nil {
		log.Panic("Encode error:", err)
	}
	encodedString := string(encodedBytes)

	payload := payload.NewPayload().Alert("🎺").MutableContent().Custom("p", encodedString)

	notification := &apns2.Notification{}
	notification.DeviceToken = token
	notification.Payload = payload

	client := apns2.NewClient(cert).Development()
	res, err := client.Push(notification)
	if err != nil {
		log.Panic("Push error:", err)
	}

	fmt.Fprintf(writer, "Sent notification to %s -> %v %v %v", token, res.StatusCode, res.ApnsID, res.Reason)
}

func main() {
	var err error
	cert, err = certificate.FromP12File("toot-relay.p12", "")
	if err != nil {
    	log.Fatal("Cert error:", err)
	}

	http.HandleFunc("/relay-to/", handler)

	if _, err := os.Stat("toot-relay.crt"); !os.IsNotExist(err) {
		log.Fatal(http.ListenAndServeTLS(":8080", "toot-relay.crt", "toot-relay.key", nil))
	} else {
		log.Fatal(http.ListenAndServe(":8080", nil))
	}
}
