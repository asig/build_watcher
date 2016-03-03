package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/tarm/serial"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

type BuildStatus struct {
	DisplayName string `json:"displayName"`
	Result      string `json:"result"`
}

type Configuration struct {
	Url      string `json:"url"`
	User     string `json:"user"`
	Password string `json:"password"`
}

var config Configuration

func fetchJenkinsStatus(buildResults chan string) {

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: tr}

	for {
		request, err := http.NewRequest("GET", config.Url, nil)
		request.SetBasicAuth(config.User, config.Password)
		response, err := httpClient.Do(request)
		if err != nil {
			log.Printf("Can't get last build status: %s", err)
			continue
		}

		contents, err := ioutil.ReadAll(response.Body)
		var buildStatus BuildStatus
		if err = json.Unmarshal(contents, &buildStatus); err != nil {
			log.Printf("Can't unmarshal response %s", contents)
			return
		}
		buildResults <- buildStatus.Result
		response.Body.Close()
		time.Sleep(60 * time.Second)
	}
}

func updateLed(buildResults chan string, port io.ReadWriteCloser) {
	port.Write([]byte("0F0\n"))
	for {
		status := <-buildResults
		switch status {
		case "SUCCESS":
			port.Write([]byte("0F0\n"))
		case "FAILURE":
			port.Write([]byte("F00\n"))
		default:
			port.Write([]byte("FF0\n"))
		}
	}
}

func pickSerialDevice() (string, error) {
	files, err := ioutil.ReadDir("/dev")
	if err != nil {
		log.Fatal(err)
		return "", errors.New("Can't read directory of /dev")
	}

	for _, file := range files {
		name := file.Name()
		if strings.HasPrefix(name, "ttyUSB") || strings.HasPrefix(name, "tty.usbserial") {
			return "/dev/" + name, nil
		}
	}
	return "", errors.New("No USB-Serial device found")
}

func loadConfiguration() {
	file, e := ioutil.ReadFile("./settings.json")
	if e != nil {
		panic(e)
	}
	if e = json.Unmarshal(file, &config); e != nil {
		panic(e)
	}
}

func main() {
	loadConfiguration()

	device, err := pickSerialDevice()
	if err != nil {
		panic(err)
	}

	c := &serial.Config{Name: device, Baud: 9600}
	port, err := serial.OpenPort(c)
	if err != nil {
		panic(err)
	}
	// Give arduino some time to reset
	time.Sleep(5 * time.Second)

	quit := make(chan interface{})
	buildResults := make(chan string)
	go fetchJenkinsStatus(buildResults)
	go updateLed(buildResults, port)
	select {
	case <-quit:
		break
	}
	port.Close()
}
