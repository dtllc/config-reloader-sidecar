package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/go-ps"
	"golang.org/x/sys/unix"
)

func main() {
	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		log.Fatal("mandatory env var CONFIG_DIR is empty, exiting")
	}

	processName := os.Getenv("PROCESS_NAME")
	if processName == "" {
		log.Fatal("mandatory env var PROCESS_NAME is empty, exiting")
	}

	verbose := false
	verboseFlag := os.Getenv("VERBOSE")
	if verboseFlag == "true" {
		verbose = true
	}

	var reloadSignal syscall.Signal
	reloadSignalStr := os.Getenv("RELOAD_SIGNAL")
	if reloadSignalStr == "" {
		log.Printf("RELOAD_SIGNAL is empty, defaulting to SIGHUP")
		reloadSignal = syscall.SIGHUP
	} else {
		reloadSignal = unix.SignalNum(reloadSignalStr)
		if reloadSignal == 0 {
			log.Fatalf("cannot find signal for RELOAD_SIGNAL: %s", reloadSignalStr)
		}
	}

	log.Printf("starting reloader with CONFIG_DIR=%s, PROCESS_NAME=%s, RELOAD_SIGNAL=%s\n", configDir, processName, reloadSignal)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if verbose {
					log.Println("event:", event)
				}
				if event.Op&fsnotify.Chmod != fsnotify.Chmod {
					log.Println("modified file:", event.Name)
					err := reloadProcesses(processName, reloadSignal)
					if err != nil {
						log.Println("error:", err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	configDirs := strings.Split(configDir, ",")
	for _, dir := range configDirs {
		err = watcher.Add(dir)
		if err != nil {
			log.Fatal(err)
		}
	}

	<-done
}

func findPIDs(process string) ([]int, error) {
	processes, err := ps.Processes()
	if err != nil {
		return nil, fmt.Errorf("failed to list processes: %v\n", err)
	}

	var similar_processes []int
	for _, p := range processes {
		if p.Executable() == process {
			log.Printf("found executable %s (pid: %d)\n", p.Executable(), p.Pid())
			similar_processes = append(similar_processes, p.Pid())
		}
	}
	if len(similar_processes) > 0 {
		return similar_processes, nil
	}


	return nil, fmt.Errorf("no process matching %s found\n it may still be starting...", process) // TODO: this should be a WARNING not an ERROR
}

func reloadProcesses(process string, signal syscall.Signal) error {
	pids, err := findPIDs(process)
	if err != nil {
		return err
	}

	log.Printf("PIDs found: %d \n", pids)

	for _, pid := range pids {
		err = syscall.Kill(pid, signal)
		if err != nil {
			return fmt.Errorf("could not send signal: %v\n", err)
		}

		log.Printf("signal %s sent to %s (pid: %d)\n", signal, process, pid)
	}
	return nil
}
