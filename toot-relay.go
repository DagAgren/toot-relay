package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
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
)

var (
	developmentClient *apns2.Client
	productionClient *apns2.Client
)

func main() {
	p12file := env("P12_FILENAME", "toot-relay.p12")
	p12base64 := env("P12_BASE64", "")
	p12password := env("P12_PASSWORD", "")

	port := env("PORT", "42069")
	tlsCrtFile := env("CRT_FILENAME", "toot-relay.crt")
	tlsKeyFile := env("KEY_FILENAME", "toot-relay.key")

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

	http.HandleFunc("/relay-to/", handler)

	if _, err := os.Stat("toot-relay.crt"); !os.IsNotExist(err) {
		log.Fatal(http.ListenAndServeTLS(":" + port, tlsCrtFile, tlsKeyFile, nil))
	} else {
		log.Fatal(http.ListenAndServe(":" + port, nil))
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
	notification.Topic = "cx.c3.toot"

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

	var client *apns2.Client
	if isProduction {
		client = productionClient
	} else {
		client = developmentClient
	}

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
			dest[4 - i] = z85digits[value % 85]
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

		for i := 0; i < suffixLength + 1; i++ {
			dest[suffixLength - i] = z85digits[value % 85]
			value /= 85
		}
	}

	return string(encodedBytes)
}
