package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/fatih/color"
	flag "github.com/spf13/pflag"
)

const (
	version = "v0.0.1"
)

type Config struct {
	ideHost    string // TCP host of the IDE
	idePort    int    // TCP port of the IDE
	listenHost string // TCP host to listen on (0.0.0.0 for all interfaces)
	listenPort int    // TCP port to listen on
	verbose    bool   // true when need to display more info
	nocolor    bool   // true when no colors are needed
}

var config Config

func main() {
	parseFlags()

	color.NoColor = config.nocolor

	displayLogo()

	address := fmt.Sprintf("%s:%d", config.ideHost, config.idePort)
	color.Set(color.FgHiGreen)
	fmt.Printf("+ Checking if connection to the IDE can be made at %s\n", address)
	ideConn, err := net.Dial("tcp", address)
	if err != nil {
		color.Set(color.FgHiRed)
		fmt.Printf("! IDE does not seem to be avaiable: %s\n", err.Error())
		fmt.Printf("  Check if you are using the correct host and port, and that the debugger in your IDE is turned on.\n")
		os.Exit(1)
	}
	_ = ideConn.Close()

	address = fmt.Sprintf("%s:%d", config.listenHost, config.listenPort)
	color.Set(color.FgHiGreen)
	fmt.Printf("+ Checking if listening port is available at %s\n", address)

	// Open listening connection
	phpServer, err := net.Listen("tcp", address)
	if err != nil {
		color.Set(color.FgHiRed)
		fmt.Printf("! Port doesnt seem to be available: %s\n", err.Error())
		os.Exit(1)
	}
	defer func() {
		_ = phpServer.Close()
	}()

	color.Set(color.FgHiGreen)
	fmt.Printf("? Set 'xdebug.client_host=%s' and 'xdebug.client_port=%d'\n", config.listenHost, config.listenPort)
	fmt.Printf("  in your php.ini configuration to connect to this proxy.\n")



	color.Set(color.FgHiGreen)
	fmt.Println("+ Waiting for incoming PHP connections")

	// Accept connections from PHP, and handle proxy between PHP and IDE
	for {
		phpConn, err := phpServer.Accept()
		if err != nil {
			log.Panic(err.Error())
		}

		color.Set(color.FgHiGreen)
		fmt.Println("- Received incoming connection from PHP")

		color.Set(color.FgHiGreen)
		fmt.Println("- Opening proxy connection to the IDE")
		address = fmt.Sprintf("%s:%d", config.ideHost, config.idePort)
		ideConn, err := net.Dial("tcp", address)
		if err != nil {
			log.Panic(err.Error())
		}

		handleProxy(phpConn, ideConn)

		fmt.Println("- Connection completed")
	}
}

func handleProxy(phpConn net.Conn, ideConn net.Conn) {
	phpChan := chanFromConn(phpConn)
	ideChan := chanFromConn(ideConn)

	defer func() {
		_ = ideConn.Close()
	}()
	defer func() {
		_ = phpConn.Close()
	}()

	for {
		select {
		case b1 := <-phpChan:
			if b1 == nil {
				color.Set(color.FgHiGreen)
				fmt.Println("! PHP connection closed")
				return
			} else {
				if config.verbose {
					fmt.Println("> ide: ", color.HiBlueString(string(b1)))
				} else {
					fmt.Printf("> ide: received %d characters\n", len(string(b1)))
				}
				_, err := ideConn.Write(b1)
				if err != nil {
					color.Set(color.FgHiRed)
					fmt.Println("! Error writing to PHP: ", err)
				}
			}
		case b2 := <-ideChan:
			if b2 == nil {
				color.Set(color.FgHiGreen)
				fmt.Println("! IDE connection closed")
				return
			} else {
				// Convert \0 to \n, as there can be multiple commands
				s := string(bytes.Replace(b2, []byte{0}, []byte{'\n'}, -1))
				s = strings.Trim(s, "\n")

				fmt.Println("> php: ", color.HiRedString(s))
				_, err := phpConn.Write(b2)
				if err != nil {
					color.Set(color.FgHiRed)
					fmt.Println("! Error writing to IDE: ", err)
				}
			}
		}
	}
}

func displayLogo() {
	logo := "         _ _\n" +
		"        | | |\n" +
		"__  ____| | |__   __ _ _ __  _ ____  ___   _\n" +
		"\\ \\/ / _` | '_ \\ / _` | '_ \\| '__\\ \\/ / | | |\n" +
		" >  < (_| | |_) | (_| | |_) | |   >  <| |_| |\n" +
		"/_/\\_\\__,_|_.__/ \\__, | .__/|_|  /_/\\_\\\\__, |\n" +
		"                  __/ | |               __/ |\n" +
		"                 |___/|_|              |___/\n"

	color.Set(color.FgHiCyan)
	fmt.Print(logo)

	color.Set(color.FgHiGreen)
	fmt.Println("+ Starting XdbgPrxy " + version + " - Joshua Thijssen (https://github.com/jaytaph)")
}

func parseFlags() {
	flag.StringVar(&config.ideHost, "ide-host", "127.0.0.1", "IP to your IDE")
	flag.IntVar(&config.idePort, "ide-port", 9003, "Port to your IDE")
	flag.StringVar(&config.listenHost, "listen-host", "127.0.0.1", "IP to listen on")
	flag.IntVar(&config.listenPort, "listen-port", 9003, "Port to listen on")
	flag.BoolVar(&config.verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&config.nocolor, "no-color", false, "No ANSI color output")

	flag.Parse()
}

func chanFromConn(conn net.Conn) chan []byte {
	c := make(chan []byte)

	go func() {
		b := make([]byte, 1024)

		for {
			n, err := conn.Read(b)
			if n > 0 {
				res := make([]byte, n)
				copy(res, b[:n])
				c <- res
			}
			if err != nil {
				c <- nil
				break
			}
		}
	}()

	return c
}
