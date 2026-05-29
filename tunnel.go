package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	maxRetriesLocal  = 16
	maxRetriesRemote = 16
	maxRetriesServer = 16
)

// dialWithRetry calls dial() up to maxRetries times, sleeping 1s between attempts.
// Returns the value on success or an error when all retries are exhausted.
func dialWithRetry[T any](maxRetries int, label string, dial func() (T, error)) (T, error) {
	for i := range maxRetries {
		v, err := dial()
		if err == nil {
			return v, nil
		}
		log.Printf("Failed to connect to %s: %s", label, err)
		if i+1 < maxRetries {
			log.Println("Retry...")
			time.Sleep(time.Second)
		}
	}
	var zero T
	return zero, fmt.Errorf("no more retries for %s", label)
}

// createLocalEndPoint binds a TCP listener on localAddr, retrying up to maxRetriesLocal times.
func createLocalEndPoint(localAddr string) net.Listener {
	l, err := dialWithRetry(maxRetriesLocal, localAddr, func() (net.Listener, error) {
		return net.Listen("tcp", localAddr)
	})
	if err != nil {
		log.Fatalf("No more retries for local end-point: %s", localAddr)
	}
	log.Printf("Listening on local address %s", localAddr)
	return l
}

// acceptClients accepts incoming connections on listener and forwards each one through
// the SSH tunnel to remoteAddr via the SSH server at serverAddr.
// Blocks forever (run in a goroutine per tunnel).
func acceptClients(listener net.Listener, config *ssh.ClientConfig, serverAddr, remoteAddr string) {
	for {
		localConn, err := listener.Accept()
		if err != nil {
			log.Printf("Accepting a client failed: %s", err)
			continue
		}
		log.Println("Client accepted.")
		go forward(localConn, config, serverAddr, remoteAddr)
	}
}

// forward establishes an SSH connection to serverAddr, opens a channel to remoteAddr,
// and pipes data between localConn and the remote end-point.
func forward(localConn net.Conn, config *ssh.ClientConfig, serverAddr, remoteAddr string) {
	defer localConn.Close()

	sshClient, err := dialWithRetry(maxRetriesServer, serverAddr, func() (*ssh.Client, error) {
		return ssh.Dial("tcp", serverAddr, config)
	})
	if err != nil {
		log.Printf("SSH server connection failed: %s", err)
		return
	}
	defer sshClient.Close()
	log.Printf("Connected to SSH server %s", serverAddr)

	remoteConn, err := dialWithRetry(maxRetriesRemote, remoteAddr, func() (net.Conn, error) {
		return sshClient.Dial("tcp", remoteAddr)
	})
	if err != nil {
		log.Printf("Remote end-point connection failed: %s", err)
		return
	}
	defer remoteConn.Close()
	log.Printf("Remote end-point %s connected.", remoteAddr)

	// Bidirectional copy. Buffered channel of 2 so both goroutines can always
	// send without blocking — prevents a goroutine leak when one side closes first.
	quit := make(chan struct{}, 2)
	go transferData(localConn, remoteConn, "Local => Remote", quit)
	go transferData(remoteConn, localConn, "Remote => Local", quit)

	// Wait for either direction to finish, then return.
	// Deferred Close() calls above will unblock the other goroutine.
	<-quit
	log.Println("At least one transfer stopped. Closing connections.")
}

// transferData copies all data from src to dst and signals quit when done.
func transferData(dst io.Writer, src io.Reader, name string, quit chan struct{}) {
	log.Printf("%s transfer started.", name)
	if _, err := io.Copy(dst, src); err != nil {
		log.Printf("%s transfer failed: %s", name, err)
	} else {
		log.Printf("%s transfer closed.", name)
	}
	quit <- struct{}{}
}

// passwordCallback returns a function that provides password for SSH auth.
func passwordCallback(password string) func() (string, error) {
	return func() (string, error) {
		return password, nil
	}
}

// keyboardInteractiveChallenge returns a handler for keyboard-interactive SSH auth.
// When the server asks exactly one question (typically "Password:"), it answers
// with the provided password.
func keyboardInteractiveChallenge(password string) ssh.KeyboardInteractiveChallenge {
	return func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		log.Printf("Keyboard-interactive: user=%s instruction=%q questions=%v", user, instruction, questions)
		switch len(questions) {
		case 0:
			return []string{}, nil
		case 1:
			return []string{password}, nil
		default:
			return nil, fmt.Errorf("SSH server asked %d keyboard-interactive questions — not supported", len(questions))
		}
	}
}
