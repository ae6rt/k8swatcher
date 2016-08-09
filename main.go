package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
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

	u, _ := url.Parse("wss://" + *addr + "/api/v1/watch/pods?watch=true&labelSelector=type=microservice")
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
				continue
			}
			var event Event
			if err := json.Unmarshal(message, &event); err != nil {
				log.Println(err)
				continue
			}

			switch event.Type {
			case "ADDED":
				for _, c := range event.Object.Spec.Containers {
					if len(c.Ports) != 1 || c.Ports[0].Name != "server-port" {
						continue
					}

					scheme := "http"
					for _, env := range c.Environment {
						if env.Name == "KEYSTORE" {
							scheme = "https"
							break
						}
					}

					log.Printf("would register %s -> %s://%s:%d\n", event.Object.Metadata.Labels["service-name"], scheme, event.Object.Status.PodIP, c.Ports[0].ContainerPort)
				}
			}
		}
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	for {
		select {
		case <-interrupt:
			log.Println("interrupt")
			// To cleanly close a connection, a client should send a close frame and wait for the server to close the connection.
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

type Event struct {
	Type   string `json:"type"`
	Object struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
		Spec struct {
			Containers []struct {
				Name  string `json:"name"`
				Image string `json:"image"`
				Ports []struct {
					Name          string `json:"name"`
					ContainerPort int    `json:"containerPort"`
					Protocol      string `json:"protocol"`
				} `json:"ports"`
				Environment []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:env"`
			} `json:"containers"`
		} `json:"spec"`
		Status struct {
			Phase  string `json:"phase"`
			PodIP  string `json:"podIP"`
			HostIP string `json:"hostIP"`
		} `json:"status"`
	} `json:"object"`
}
