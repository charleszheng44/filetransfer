package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/user"
	"path"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	defaultPort              = 8844
	defaultListTimeoutSecs   = 3
	defaultLookupTimeoutSecs = 1
	service                  = "_ftr._tcp"
	domain                   = "local."
	letters                  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

func exitWithError(code int, format string, v ...any) {
	fmt.Printf(format, v...)
	os.Exit(code)
}

func main() {
	args := os.Args
	if len(args) < 2 {
		log.Fatal("Subcommand is not provided")
	}

	subCommand := args[1]
	switch subCommand {
	case "daemon":
		runDaemon()
	case "list":
		runList()
	case "send":
		runSend(args[2:])
	default:
		exitWithError(1, "Unrecognized subcommand: %s", subCommand)
	}
}

func defaultDropDir() string {
	user, err := user.Current()
	if err != nil {
		exitWithError(1, "Failed to get the current user: %v", err)
	}
	homeDir := user.HomeDir
	if homeDir == "" {
		exitWithError(1, "HomeDir of the current user is empty")
	}
	return path.Join(homeDir, "Downloads")
}

func trimHostNameSuffix(fullName string) string {
	if i := strings.Index(fullName, "."); i > 0 {
		return fullName[:i]
	}
	return fullName
}

func randomPassKey(n int) string {
	result := make([]byte, n)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			exitWithError(1, "Failed to generate random passkey: %v", err)
		}
		result[i] = letters[num.Int64()]
	}
	return string(result)
}

func runDaemon() {
	daemonCmd := flag.NewFlagSet("daemon", flag.ExitOnError)
	daemonCmd.SetOutput(os.Stdout)
	hostName, err := os.Hostname()
	if err != nil {
		exitWithError(1, "Failed to get the hostname: %v", err)
	}
	name := trimHostNameSuffix(*daemonCmd.String("name", hostName, "the name for the host"))
	port := daemonCmd.Int("port", defaultPort, "the port the server will listen at")
	dropDir := daemonCmd.String("drop-dir", defaultDropDir(), "the path to the default drop dir")
	passKey := daemonCmd.String("passkey", randomPassKey(6), "the passkey used to authn the file transfer")

	// All available ip addresses will be appended to the entry automatically
	rvrSvr, err := zeroconf.Register(
		name, service, domain, *port,
		// the meta info used as the TXT record
		[]string{*dropDir}, nil,
	)
	if err != nil {
		exitWithError(1, "Failed to start the receiver server: %v", err)
	}
	defer rvrSvr.Shutdown()
	fmt.Printf("Advertise within the network with name %s, port %d and passkey %s\n", name, *port, *passKey)

	//TODO(charleszheng44): start the http server receiving files
}

func runList() {
	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	listCmd.SetOutput(os.Stdout)
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		exitWithError(1, "Failed to get the resolver: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultListTimeoutSecs*time.Second)
	defer cancel()

	entries := make(chan *zeroconf.ServiceEntry)

	go func() {
		fmt.Printf(
			"%-20s, %-20s, %-5s, %-20s\n",
			"HostName", "IPv4", "Port", "DropDir",
		)
		for e := range entries {
			fmt.Printf(
				"%-20s, %-20s, %-5d, %-20s\n",
				e.HostName, e.AddrIPv4[0], e.Port, e.Text[0],
			)
		}
	}()

	if err := resolver.Browse(ctx, service, domain, entries); err != nil {
		exitWithError(1, "Failed to list peers: %v", err)
	}
	<-ctx.Done()
}

func runSend(args []string) {
	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
	sendCmd.SetOutput(os.Stdout)
	_ = sendCmd.String("psk", "", "pre-shared passkey")
	if err := sendCmd.Parse(args); err != nil {
		exitWithError(1, "Send command failed: %v", err)
	}
	pos := sendCmd.Args()
	if len(pos) != 2 {
		fmt.Println("Usage: ftr send --psk <key> <path> <peer>")
		os.Exit(1)
	}

	_, peer := pos[0], pos[1]

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		exitWithError(1, "failed to get the peer resolver: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultLookupTimeoutSecs*time.Second)
	defer cancel()

	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		e := <-entries
		fmt.Printf("Found the peer %s with ip %s and port %d", e.HostName, e.AddrIPv4[0], e.Port)
		cancel()
	}()

	if err := resolver.Lookup(ctx, peer, service, domain, entries); err != nil {
		exitWithError(1, "Failed to find the peer: %v", err)
	}
	<-ctx.Done()
}
