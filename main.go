package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"mime/multipart"
	"net/http"
	"os"
	"os/user"
	"path"
	"path/filepath"
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
	passKeyHeader          = "X-Ftr-Passkey"
	fileTypeHeader         = "X-Ftr-File-Type"
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

func isDirectory(header http.Header) bool {
	if header == nil {
		return false
	}
	fileType := header.Get(fileTypeHeader)

	if fileType == "" {
		return false
	}

	if fileType == "dir" {
		return true
	}

	if fileType == "file" {
		return false
	}

	return false
}

func zipTar(src string) (string, error) {
	tarball := src + ".tar.gz"
	file, err := os.Create(tarball)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	err = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		// return on any error
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(filepath.Dir(src), path)
		if err != nil {
			return err
		}
		// ignore the top-level directory
		if relPath == "." {
			return nil
		}
		name := filepath.ToSlash(relPath)
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = name

		// ignore the file if it's a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		if d.IsDir() {
			header.Typeflag = tar.TypeDir

			// ensure the directory can be accessible on the receiver side
			if header.Mode&0400 == 0 {
				header.Mode |= 0100
			}
			if header.Mode&0040 == 0 {
				header.Mode |= 0010
			}
			if header.Mode&0004 == 0 {
				header.Mode |= 0001
			}
			return tw.WriteHeader(header)
		}

		// for regular files
		header.Typeflag = tar.TypeReg
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := io.Copy(tw, f); err != nil {
			return err
		}
		return nil
	})
	return tarball, err
}

func unzipUntar(src string) error {
	var dst string
	if strings.HasSuffix(src, ".tar.gz") {
		dst = strings.TrimSuffix(src, "tar.gz")
	} else if strings.HasSuffix(src, ".tgz") {
		dst = strings.TrimSuffix(src, "tgz")
	} else {
		return errors.New("the file is not a tarball")
	}

	file, err := os.Open(src)
	if err != nil {
		return err
	}

	gr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			dirPath := path.Join(dst, header.Name)
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			filePath := path.Join(dst, header.Name)
			if err := os.MkdirAll(path.Dir(filePath), 0755); err != nil {
				return err
			}
			outFile, err := os.Create(filePath)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		default:
			return fmt.Errorf("Unrecognized tar entry type: %v", header.Typeflag)
		}
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

		// untar if the file is a tarball of a directory
		if isDirectory(r.Header) {
			// untar the file
			if err := unzipUntar(dstPath); err != nil {
				http.Error(w, "Failed to unzip and untar the file on server", http.StatusInternalServerError)
				return
			}
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

func sendFile(src, key, addr string, port int) error {
	fi, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat the source file: %v", err)
	}
	if fi.IsDir() {
		src, err = zipTar(src)
		if err != nil {
			return fmt.Errorf("failed to zip and tar the source directory: %v", err)
		}
		defer os.Remove(src)
	}

	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open the source file: %v", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", path.Base(src))
	if err != nil {
		return fmt.Errorf("failed to create form file: %v", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy the file content to form: %v", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close the multipart writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s:%d/upload", addr, port), body)
	if err != nil {
		return fmt.Errorf("failed to create the http request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set(passKeyHeader, key)
	req.Header.Set(fileTypeHeader, "file")
	if fi.IsDir() {
		req.Header.Set(fileTypeHeader, "dir")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send the http request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send the file, server returned status: %s", resp.Status)
	}
	fmt.Println("File sent successfully")
	return nil
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

	src, peer := pos[0], pos[1]
	key := sendCmd.Lookup("psk").Value.String()

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		exitWithError(1, "failed to get the peer resolver: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultLookupTimeoutMs*time.Millisecond)
	defer cancel()

	entries := make(chan *zeroconf.ServiceEntry)
	transferCompleted := make(chan struct{})
	go func() {
		e := <-entries
		fmt.Printf("Found the peer %s with ip %s and port %d", e.HostName, e.AddrIPv4[0], e.Port)
		fmt.Println("Start sending the file...")
		if err := sendFile(src, key, e.AddrIPv4[0].String(), e.Port); err != nil {
			exitWithError(1, "Failed to send the file: %v", err)
		}
		close(transferCompleted)
	}()

	if err := resolver.Lookup(ctx, peer, service, domain, entries); err != nil {
		exitWithError(1, "Failed to find the peer: %v", err)
	}
	<-transferCompleted
}
