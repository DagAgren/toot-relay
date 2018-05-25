package main

import (
	"bytes"
	"crypto/tls"
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

var cert tls.Certificate

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

func handler(writer http.ResponseWriter, request *http.Request) {
	components := strings.Split(request.URL.Path, "/")

	notification := &apns2.Notification{}
	notification.DeviceToken = components[2]

	buffer := new(bytes.Buffer)
	buffer.ReadFrom(request.Body)
	encodedString := encode85(buffer.Bytes())
	payload := payload.NewPayload().Alert("🎺").MutableContent().Custom("p", encodedString)

	if len(components) > 3 {
		payload.Custom("x", strings.Join(components[3:], "/"))
	}

	notification.Payload = payload

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
