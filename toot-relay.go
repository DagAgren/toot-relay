package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"strconv"
	"time"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"github.com/tilinna/z85"
)

var cert tls.Certificate

func handler(writer http.ResponseWriter, request *http.Request) {
	components := strings.Split(request.URL.Path, "/")

	notification := &apns2.Notification{}
	notification.DeviceToken = components[2]

	buffer := new(bytes.Buffer)
    buffer.ReadFrom(request.Body)
    body := buffer.Bytes()

	encodedBytes := make([]byte, z85.EncodedLen(len(body)))
	_, err := z85.Encode(encodedBytes, body)
	if err != nil {
		writer.WriteHeader(500)
		fmt.Fprintln(writer, "Encode error:", err)
		log.Println("Encode error:", err)
		return
	}
	encodedString := string(encodedBytes)

	payload := payload.NewPayload().Alert("🎺").MutableContent().Custom("p", encodedString)

	if len(components) > 3 {
		payload.Custom("x", strings.Join(components[3:], "/"))
	}

	notification.Payload = payload

	if seconds := request.Header.Get("TTL"); seconds != "" {
		if ttl, err := strconv.Atoi(seconds); err == nil {
			notification.Expiration = time.Now().Add(time.Duration(ttl) * time.Second)
		}
	}

	if topic := request.Header.Get("Topic"); topic != "" {
		notification.CollapseID = topic
	}

	switch request.Header.Get("Urgency") {
		case "very-low", "low":
			notification.Priority = apns2.PriorityLow
		default:
			notification.Priority = apns2.PriorityHigh
	}

	client := apns2.NewClient(cert).Development()
	res, err := client.Push(notification)
	if err != nil {
		writer.WriteHeader(500)
		fmt.Fprintln(writer, "Push error:", err)
		log.Println("Push error:", err)
		return
	}

	if res.Sent() {
		writer.Header().Add("Location", fmt.Sprintf("https://not-supported/%v", res.ApnsID))
		writer.WriteHeader(201)
		log.Printf("Sent notification to %s -> %v %v %v", notification.DeviceToken, res.StatusCode, res.ApnsID, res.Reason)
		log.Println("Expiration:", notification.Expiration)
		log.Println("Priority:", notification.Priority)
		log.Println("CollapseID:", notification.CollapseID)
	} else {
		writer.WriteHeader(res.StatusCode)
		fmt.Fprintln(writer, res.Reason)
		log.Printf("Failed to send: %v %v %v\n", res.StatusCode, res.ApnsID, res.Reason)
	}
}

func main() {
	var err error
	cert, err = certificate.FromP12File("toot-relay.p12", "")
	if err != nil {
    	log.Fatal("Cert error:", err)
	}

	http.HandleFunc("/relay-to/", handler)

	if _, err := os.Stat("toot-relay.crt"); !os.IsNotExist(err) {
		log.Fatal(http.ListenAndServeTLS(":42069", "toot-relay.crt", "toot-relay.key", nil))
	} else {
		log.Fatal(http.ListenAndServe(":42069", nil))
	}
}
