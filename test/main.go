package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

var (
	Servers = []string{"http://localhost:8080", "http://localhost:8082", "http://localhost:8084"}
)

func main() {
	if _, err := os.Stat("docker-compose.yml"); os.IsNotExist(err) {
		log.Fatal("You need to be running this in the same directory as docker-compose.yml")
	} else if err != nil {
		log.Fatal(err)
	}

	go runCommand("docker-compose", strings.Split("up --remove-orphans --force-recreate --build", " "))

	time.Sleep(3 * time.Minute)

	for _, s := range Servers {
		val := time.Now().String()
		if _, err := http.Post(fmt.Sprintf("%s/key/test", s), "application/x-www-form-urlencoded", strings.NewReader(val)); err != nil {
			log.Fatal(err)
		}

		// Now make sure we can read it everywhere.
		for _, t := range Servers {
			resp, err := http.Get(fmt.Sprintf("%s/key/test", t))
			if err != nil {
				log.Fatal(err)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Fatal(err)
			}

			if val != string(body) {
				log.Printf("Error on Server %s: %q != %q", t, val, string(body))
			}
		}

		time.Sleep(10 * time.Second)
	}

	// The possible values here can be seen from "docker-compose ps --services"
	go runCommand("docker-compose", strings.Split("restart kv_4", " "))
	time.Sleep(5 * time.Second)

	// Now a test with a server down
	val := "test test test"
	if _, err := http.Post(fmt.Sprintf("%s/key/test2", Servers[0]), "application/x-www-form-urlencoded", strings.NewReader(val)); err != nil {
		log.Fatal(err)
	}

	// Sleep for data to replicate
	time.Sleep(5 * time.Second)
	resp, err := http.Get(fmt.Sprintf("%s/key/test2", Servers[1]))
	if err != nil {
		log.Fatal(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	if val != string(body) {
		log.Printf("Error on down test: %q != %q", val, string(body))
	}

	runCommand("docker-compose", []string{"down"})
}

func logBuffer(prefix string, in io.ReadCloser) {
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		log.Printf("%s: %s", prefix, scanner.Text())
	}
}

func runCommand(command string, args []string) {
	cmd := exec.Command(command, args...)
	log.Printf("running %+v", cmd)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	go logBuffer("stderr", stderr)
	go logBuffer("stdout", stdout)

	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
}
