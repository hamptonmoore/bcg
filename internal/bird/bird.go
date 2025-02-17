package bird

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/natesales/pathvector/internal/util"
)

func read(reader io.Reader) (string, error) {
	// TODO: This buffer isn't a good solution, and might not fit the full response from BIRD
	buf := make([]byte, 16384)
	n, err := reader.Read(buf[:])

	if err != nil {
		return "", fmt.Errorf("BIRD read: %v", err)
	}

	return string(buf[:n]), nil // nil error
}

// RunCommand runs a BIRD command
func RunCommand(command string, socket string) (string, error) {
	log.Debugln("Connecting to BIRD socket")
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return "", err
	}
	//noinspection GoUnhandledErrorResult
	defer conn.Close()

	log.Println("Connected to BIRD socket")
	resp, err := read(conn)
	if err != nil {
		return "", err
	}
	log.Debugf("BIRD init response: %s", resp)

	log.Debugf("Sending BIRD command: %s", command)
	_, err = conn.Write([]byte(strings.Trim(command, "\n") + "\n"))
	log.Debugf("Sent BIRD command: %s", command)
	if err != nil {
		return "", err
	}

	log.Debugln("Reading from socket")
	resp, err = read(conn)
	if err != nil {
		return "", err
	}
	log.Debugln("Done reading from socket")

	return resp, nil // nil error
}

// Validate checks if the cached configuration is syntactically valid
func Validate(binary string, cacheDir string) {
	birdCmd := exec.Command(binary, "-c", "bird.conf", "-p")
	birdCmd.Dir = cacheDir
	birdCmd.Stdout = os.Stdout
	birdCmd.Stderr = os.Stderr
	if err := birdCmd.Run(); err != nil {
		log.Fatalf("BIRD config validation: %v", err)
	}
	log.Infof("BIRD config validation passed")
}

// MoveCacheAndReconfigure moves cached files to the production BIRD directory and reconfigures
func MoveCacheAndReconfigure(birdDirectory string, cacheDirectory string, birdSocket string, noConfigure bool) {
	// Remove old configs
	birdConfigFiles, err := filepath.Glob(path.Join(birdDirectory, "AS*.conf"))
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range birdConfigFiles {
		log.Debugf("Removing old BIRD config file %s", f)
		if err := os.Remove(f); err != nil {
			log.Fatalf("Removing old BIRD config files: %v", err)
		}
	}

	// Copy from cache to bird config
	files, err := filepath.Glob(path.Join(cacheDirectory, "*.conf"))
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		fileNameParts := strings.Split(f, "/")
		fileNameTail := fileNameParts[len(fileNameParts)-1]
		newFileLoc := path.Join(birdDirectory, fileNameTail)
		log.Debugf("Moving %s to %s", f, newFileLoc)
		if err := util.MoveFile(f, newFileLoc); err != nil {
			log.Fatalf("Moving cache file to bird directory: %v", err)
		}
	}

	if !noConfigure {
		log.Infoln("Reconfiguring BIRD")
		resp, err := RunCommand("configure", birdSocket)
		if err != nil {
			log.Fatal(err)
		}
		// Print bird output as multiple lines
		for _, line := range strings.Split(strings.Trim(resp, "\n"), "\n") {
			log.Printf("BIRD response (multiline): %s", line)
		}
	}
}

// Reformat takes a BIRD config file as a string and outputs a nicely formatted version as a string
func Reformat(input string) string {
	formatted := ""
	for _, line := range strings.Split(input, "\n") {
		if strings.HasSuffix(line, "{") || strings.HasSuffix(line, "[") {
			formatted += "\n"
		}

		if !func(input string) bool {
			for _, chr := range input {
				if string(chr) != " " {
					return false
				}
			}
			return true
		}(line) {
			formatted += line + "\n"
		}
	}
	return formatted
}
