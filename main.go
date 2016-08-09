package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "kubernetes:6443", "http service address")
var caCert = flag.String("ca-cert-file", "ca.crt", "CA cert")
var username = flag.String("username", "", "master username")
var password = flag.String("password", "", "master password")

func main() {
	flag.Parse()
	log.SetFlags(0)

	certs := x509.NewCertPool()
	data, err := ioutil.ReadFile(*caCert)
	if err != nil {
		log.Fatal(err)
	}
	certs.AppendCertsFromPEM(data)

	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{RootCAs: certs}

	u, _ := url.Parse("wss://" + *addr + "/api/v1/watch/pods?watch=true")
	log.Printf("connecting to %s", u.String())

	c, resp, err := dialer.Dial(u.String(), http.Header{
		"Origin":        []string{"https://" + *addr},
		"Authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte(*username+":"+*password))},
	})
	if err != nil {
		log.Fatalf("response %+v: err: %s", resp, err.Error())
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer c.Close()
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
		}
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	for {
		select {
		case <-interrupt:
			log.Println("interrupt")
			// To cleanly close a connection, a client should send a close
			// frame and wait for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			c.Close()
			return
		}
	}
}
