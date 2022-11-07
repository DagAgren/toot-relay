package main

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
	"golang.org/x/net/http2"
)

type Message struct {
	isProduction bool
	notification *apns2.Notification
}

var (
	developmentClient *apns2.Client
	productionClient  *apns2.Client
	topic             string
	messageChan       chan *Message
)

const (
	maxWorkers     = 4
	maxQueueLength = 1024
)

func worker(workerId int) {
	log.Printf("starting worker %d", workerId)
	defer log.Printf("stopping worker %d", workerId)

	var client *apns2.Client

	for msg := range messageChan {
		if msg.isProduction {
			client = productionClient
		} else {
			client = developmentClient
		}

		res, err := client.Push(msg.notification)

		if err != nil {
			log.Println("Push error:", err)
			continue
		}

		if res.Sent() {
			log.Printf("Sent notification to %s -> %v %v %v", msg.notification.DeviceToken, res.StatusCode, res.ApnsID, res.Reason)
			log.Println("Expiration:", msg.notification.Expiration)
			log.Println("Priority:", msg.notification.Priority)
			log.Println("CollapseID:", msg.notification.CollapseID)
		} else {
			log.Printf("Failed to send: %v %v %v\n", res.StatusCode, res.ApnsID, res.Reason)
		}
	}
}

func main() {
	topic = env("TOPIC", "cx.c3.toot")
	p12file := env("P12_FILENAME", "toot-relay.p12")
	p12base64 := env("P12_BASE64", "")
	p12password := env("P12_PASSWORD", "")

	port := env("PORT", "42069")
	tlsCrtFile := env("CRT_FILENAME", "toot-relay.crt")
	tlsKeyFile := env("KEY_FILENAME", "toot-relay.key")
	// CA_FILENAME can be set to a file that contains PEM encoded certificates that will be
	// used as the sole root CAs when connecting to the Apple Notification Service API.
	// If unset, the system-wide certificate store will be used.
	caFile := env("CA_FILENAME", "")
	var rootCAs *x509.CertPool

	if caPEM, err := ioutil.ReadFile(caFile); err == nil {
		rootCAs = x509.NewCertPool()
		if ok := rootCAs.AppendCertsFromPEM(caPEM); !ok {
			log.Fatalf("CA file %s specified but no CA certificates could be loaded\n", caFile)
		}
	}

	if p12base64 != "" {
		bytes, err := base64.StdEncoding.DecodeString(p12base64)
		if err != nil {
			log.Fatal("Base64 decoding error: ", err)
		}

		cert, err := certificate.FromP12Bytes(bytes, p12password)
		if err != nil {
			log.Fatal("Error parsing certificate: ", err)
		}

		developmentClient = apns2.NewClient(cert).Development()
		productionClient = apns2.NewClient(cert).Production()
	} else {
		cert, err := certificate.FromP12File(p12file, p12password)
		if err != nil {
			log.Fatal("Error loading certificate file: ", err)
		}

		developmentClient = apns2.NewClient(cert).Development()
		productionClient = apns2.NewClient(cert).Production()
	}

	if rootCAs != nil {
		developmentClient.HTTPClient.Transport.(*http2.Transport).TLSClientConfig.RootCAs = rootCAs
		productionClient.HTTPClient.Transport.(*http2.Transport).TLSClientConfig.RootCAs = rootCAs
	}

	http.HandleFunc("/relay-to/", handler)

	messageChan = make(chan *Message, maxQueueLength)
	for i := 1; i <= maxWorkers; i++ {
		go worker(i)
	}

	if _, err := os.Stat(tlsCrtFile); !os.IsNotExist(err) {
		log.Fatal(http.ListenAndServeTLS(":"+port, tlsCrtFile, tlsKeyFile, nil))
	} else {
		log.Fatal(http.ListenAndServe(":"+port, nil))
	}
}

func handler(writer http.ResponseWriter, request *http.Request) {
	components := strings.Split(request.URL.Path, "/")

	if len(components) < 4 {
		writer.WriteHeader(500)
		fmt.Fprintln(writer, "Invalid URL path:", request.URL.Path)
		log.Println("Invalid URL path:", request.URL.Path)
		return
	}

	isProduction := components[2] == "production"

	notification := &apns2.Notification{}
	notification.DeviceToken = components[3]

	buffer := new(bytes.Buffer)
	buffer.ReadFrom(request.Body)
	encodedString := encode85(buffer.Bytes())
	payload := payload.NewPayload().Alert("🎺").MutableContent().ContentAvailable().Custom("p", encodedString)

	if len(components) > 4 {
		payload.Custom("x", strings.Join(components[4:], "/"))
	}

	notification.Payload = payload
	notification.Topic = topic

	switch request.Header.Get("Content-Encoding") {
	case "aesgcm":
		if publicKey, err := encodedValue(request.Header, "Crypto-Key", "dh"); err == nil {
			payload.Custom("k", publicKey)
		} else {
			writer.WriteHeader(500)
			fmt.Fprintln(writer, "Error retrieving public key:", err)
			log.Println("Error retrieving public key:", err)
			return
		}

		if salt, err := encodedValue(request.Header, "Encryption", "salt"); err == nil {
			payload.Custom("s", salt)
		} else {
			writer.WriteHeader(500)
			fmt.Fprintln(writer, "Error retrieving salt:", err)
			log.Println("Error retrieving salt:", err)
			return
		}
	//case "aes128gcm": // No further headers needed. However, not implemented on client side so return 415.
	default:
		writer.WriteHeader(415)
		fmt.Fprintln(writer, "Unsupported Content-Encoding:", request.Header.Get("Content-Encoding"))
		log.Println("Unsupported Content-Encoding:", request.Header.Get("Content-Encoding"))
		return
	}

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

	messageChan <- &Message{isProduction, notification}

	// always reply w/ success, since we don't know how apple responded
	writer.WriteHeader(201)
}

func env(name, defaultValue string) string {
	if value, isPresent := os.LookupEnv(name); isPresent {
		return value
	} else {
		return defaultValue
	}
}

func encodedValue(header http.Header, name, key string) (string, error) {
	keyValues := parseKeyValues(header.Get(name))
	value, exists := keyValues[key]
	if !exists {
		return "", errors.New(fmt.Sprintf("Value %s not found in header %s", key, name))
	}

	bytes, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}

	return encode85(bytes), nil
}

func parseKeyValues(values string) map[string]string {
	f := func(c rune) bool {
		return c == ';'
	}

	entries := strings.FieldsFunc(values, f)

	m := make(map[string]string)
	for _, entry := range entries {
		parts := strings.Split(entry, "=")
		m[parts[0]] = parts[1]
	}

	return m
}

var z85digits = []byte("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ.-:+=^!/*?&<>()[]{}@%$#")

func encode85(bytes []byte) string {
	numBlocks := len(bytes) / 4
	suffixLength := len(bytes) % 4

	encodedLength := numBlocks * 5
	if suffixLength != 0 {
		encodedLength += suffixLength + 1
	}

	encodedBytes := make([]byte, encodedLength)

	src := bytes
	dest := encodedBytes
	for block := 0; block < numBlocks; block++ {
		value := binary.BigEndian.Uint32(src)

		for i := 0; i < 5; i++ {
			dest[4-i] = z85digits[value%85]
			value /= 85
		}

		src = src[4:]
		dest = dest[5:]
	}

	if suffixLength != 0 {
		value := 0

		for i := 0; i < suffixLength; i++ {
			value *= 256
			value |= int(src[i])
		}

		for i := 0; i < suffixLength+1; i++ {
			dest[suffixLength-i] = z85digits[value%85]
			value /= 85
		}
	}

	return string(encodedBytes)
}
