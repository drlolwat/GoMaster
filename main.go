package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	onlineAccounts = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "online_accounts",
		Help: "Tracks the number of online accounts per user",
	}, []string{"user_id", "agent_id"})

	onlineAgents = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "online_agents",
		Help: "Tracks the number of online agents on the BotBuddy network",
	}, []string{"agent_id", "user_id"})

	statuses = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bot_statuses",
		Help: "Tracks bot statuses across the BotBuddy network",
	}, []string{"user_id", "agent_id", "status", "script"})

	wrapperRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "wrapper_requests",
		Help: "Tracks the number of requests made by the BotBuddy wrapper",
	}, []string{"user_id", "agent_id"})

	jagexRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jagex_requests",
		Help: "Tracks the number of requests made to link Jagex accounts",
	}, []string{"user_id", "agent_id"})
)

func main() {
	fmt.Println("    ____        __  ____            __    __     ")
	fmt.Println("   / __ )____  / /_/ __ )__  ______/ /___/ /_  __")
	fmt.Println("  / __  / __ \\/ __/ __  / / / / __  / __  / / / /")
	fmt.Println(" / /_/ / /_/ / /_/ /_/ / /_/ / /_/ / /_/ / /_/ / ")
	fmt.Println("/_____/\\____/\\__/_____/\\__,_/\\__,_/\\__,_/\\__, /  ")
	fmt.Println("                                        /____/   ")
	fmt.Println()
	fmt.Println("BotBuddy Master Server")
	fmt.Println("Developed by the team at https://botbuddy.net")
	fmt.Println()

	go NewServer("8888", handleWebConnection)

	listener, err := net.Listen("tcp", ":"+"7888")
	if err != nil {
		log.Fatal(err)
	}

	dg, err := discordgo.New("Bot " + BotToken)
	if err != nil {
		fmt.Println("Error creating Discord session:", err)
		return
	}

	go func() {
		err = dg.Open()
		if err != nil {
			fmt.Println("Error opening connection to Discord:", err)
			return
		}

		dallE = false
		dg.AddHandler(messageCreate)
	}()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatal(err)
			}
			go handleAgentConnection(conn)
		}
	}()

	http.Handle("/", promhttp.Handler())
	go func() {
		err := http.ListenAndServe(":2112", nil)
		if err != nil {
			panic(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	go listener.Close()
	err = dg.Close()
	if err != nil {
		return
	}
	time.Sleep(time.Millisecond * 100)
}

func NewServer(port string, handleConnection func(net.Conn)) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConnection(conn)
	}
}
