package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
)

const webApiUrl = "http://nginx/api/"

type webHandlerMap map[string]func(net.Conn, string) error

var webMessageHandlers webHandlerMap

func init() {
	webMessageHandlers = webHandlerMap{
		"startBot":               startBot,
		"stopBot":                stopBot,
		"getRunningBotsByClient": getRunningBotsByClient,
	}
}

func handleWebConnection(conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {

		}
	}(conn)

	reader := bufio.NewReader(conn)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Println("Error reading from connection:", err)
			return
		}

		parts := strings.SplitN(line, "\r", 2)
		if len(parts) != 2 {
			log.Println("Invalid packet format")
			return
		}

		header := parts[0]
		data := parts[1]

		if handler, ok := webMessageHandlers[header]; ok {
			err := handler(conn, data)
			if err != nil {
				log.Println("Error handling message:", err)
				return
			}
		} else {
			log.Println("Unknown header:", header)
		}
	}
}

type BotConfig struct {
	ServerId string `json:"serverId"`
}

func startBot(conn net.Conn, data string) error {
	var config BotConfig
	err := json.Unmarshal([]byte(data), &config)
	if err != nil {
		log.Println("Error decoding JSON:", err)
		return err
	}

	if config.ServerId == "" {
		_, err := conn.Write([]byte("false\n"))
		if err != nil {
			return err
		}
		return errors.New("tried to start a bot with an empty serverId")
	}

	agent := getAgent(config.ServerId)
	if agent.UUID == "" {
		_, err := conn.Write([]byte("false\n"))
		if err != nil {
			return err
		}
		return errors.New("tried to start a bot without a UUID")
	}

	sendEncryptedPacket(agent.Conn, "startBot", data, agent.EncryptionKey)
	_, err = conn.Write([]byte("true\n"))
	if err != nil {
		return err
	}

	return nil
}

type StopBot struct {
	ServerId string `json:"serverId"`
}

func stopBot(conn net.Conn, data string) error {
	var config StopBot
	err := json.Unmarshal([]byte(data), &config)
	if err != nil {
		log.Println("Error decoding JSON:", err)
		return err
	}

	if config.ServerId == "" {
		_, err := conn.Write([]byte("false\n"))
		if err != nil {
			return err
		}
		return errors.New("tried to stop a bot with an empty serverId")
	}

	agent := getAgent(config.ServerId)
	if agent.UUID == "" {
		_, err := conn.Write([]byte("false\n"))
		if err != nil {
			return err
		}
		return errors.New("tried to stop a bot without a UUID")
	}

	sendEncryptedPacket(agent.Conn, "stopBot", data, agent.EncryptionKey)
	_, err = conn.Write([]byte("true\n"))
	if err != nil {
		return err
	}
	return nil
}

type GetRunningBotsData struct {
	CustomerId int `json:"customerId"`
}

func getRunningBotsByClient(_ net.Conn, data string) error {
	var botData GetRunningBotsData
	err := json.Unmarshal([]byte(data), &botData)
	if err != nil {
		log.Println("Error decoding JSON:", err)
		return err
	}

	agents := getAgentsByCustomerId(botData.CustomerId)
	for _, agent := range agents {
		if agent.UUID == "" {
			continue
		}
		sendEncryptedPacket(agent.Conn, "listRunningBots", "{}", agent.EncryptionKey)
	}
	return nil
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
