package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/user"
	"path"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	defaultPort            = 8844
	defaultListTimeoutSecs = 3
	defaultLookupTimeoutMs = 100
	service                = "_ftr._tcp"
	domain                 = "local."
	letters                = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	passKeyHeader          = "Ftr-Passkey"
)

func exitWithError(code int, format string, v ...any) {
	fmt.Printf(format, v...)
	os.Exit(code)
}

func main() {
	args := os.Args
	if len(args) < 2 {
		exitWithError(1, "Subcommand is not provided")
	}

	subCommand := args[1]
	switch subCommand {
	case "join":
		runJoin()
	case "list":
		runList()
	case "help":
		runHelp()
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

func runHelp() {
	fmt.Println(
		"Usage:\n",
		"    Join the network: `ftr join --name <name> --port <port> --dropdir <path-to-dir> --key <key>`\n",
		"    List all peers: `ftr list `\n",
		"    Send file to peer: `ftr send --key <key> file peer`",
	)
}

func runJoin() {
	joinCmd := flag.NewFlagSet("join", flag.ExitOnError)
	joinCmd.SetOutput(os.Stdout)
	hostName, err := os.Hostname()
	if err != nil {
		exitWithError(1, "Failed to get the hostname: %v", err)
	}
	name := trimHostNameSuffix(*joinCmd.String("name", hostName, "the name for the host"))
	port := joinCmd.Int("port", defaultPort, "the port the server will listen at")
	dropDir := joinCmd.String("dropdir", defaultDropDir(), "the path to the default drop dir")
	passKey := joinCmd.String("key", randomPassKey(6), "the pre-shared key used to authn the file transfer")

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
	fmt.Printf("Advertise within the network with name %s, port %d and key %s\n", name, *port, *passKey)
	errChan := make(chan error)
	go startReceiverServer(*port, *dropDir, *passKey, errChan)
	if err := <-errChan; err != nil {
		exitWithError(1, "Receiver server error: %v", err)
	}
}

func mkDirIfNotExist(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("The directory %s does not exist, creating it\n", dir)
			return os.MkdirAll(dir, 0755)
		}
		return err
	}
	return nil
}

func getFileDropHandler(dropDir, passKey string) (http.HandlerFunc, error) {
	if dropDir == "" {
		return nil, errors.New("the drop dir is empty")
	}
	if passKey == "" {
		return nil, errors.New("the passkey is empty")
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Failed to get the file from form", http.StatusBadRequest)
			return
		}
		fileName := path.Base(header.Filename)
		if fileName == "" || fileName == "." || fileName == ".." {
			http.Error(w, "Invalid file name", http.StatusBadRequest)
			return
		}
		defer file.Close()

		dstPath := path.Join(dropDir, fileName)
		if _, err := os.Stat(dstPath); err == nil {
			http.Error(w, "File already exists", http.StatusConflict)
			return
		}

		dst, err := os.Create(path.Join(dropDir, fileName))
		if err != nil {
			http.Error(w, "Failed to create the file on server", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, "Failed to save the file on server", http.StatusInternalServerError)
			return
		}

	}, nil
}

func authMiddleware(passKey string, next http.Handler) (http.Handler, error) {
	if passKey == "" {
		return nil, errors.New("the passkey is empty")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(passKeyHeader)
		if key != passKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}), nil
}

func startReceiverServer(port int, dropDir, passKey string, errChan chan<- error) {
	if err := mkDirIfNotExist(dropDir); err != nil {
		errChan <- fmt.Errorf("failed to create the drop dir %s: %v", dropDir, err)
		return
	}
	handler, err := getFileDropHandler(dropDir, passKey)
	if err != nil {
		errChan <- fmt.Errorf("failed to get the file drop handler: %v", err)
		return
	}

	handlerWithAuth, err := authMiddleware(passKey, handler)
	if err != nil {
		errChan <- fmt.Errorf("failed to get the auth middleware: %v", err)
		return
	}

	// Start the HTTP server at all interfaces with the specified port
	if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), handlerWithAuth); err != nil {
		errChan <- fmt.Errorf("failed to start the http server: %v", err)
		return
	}
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
			"%-20s %-15s %-5s %-20s\n",
			"HostName", "IPv4", "Port", "DropDir",
		)
		for e := range entries {
			fmt.Printf(
				"%-20s %-15s %-5d %-20s\n",
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
	ctx, cancel := context.WithTimeout(context.Background(), defaultLookupTimeoutMs*time.Millisecond)
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
