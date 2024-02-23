package main

import (
	"bytes"
	"io/ioutil"
	"net"
	"net/http"
)

const webApiUrl = "https://botbuddy.net/api/"

func handleWebConnection(conn net.Conn) {
}

func sendWebRequest(apiType string, data string) string {
	client := &http.Client{}

	req, err := http.NewRequest("POST", webApiUrl+apiType, bytes.NewBuffer([]byte(data)))
	if err != nil {
		return ""
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	return string(body)
}
