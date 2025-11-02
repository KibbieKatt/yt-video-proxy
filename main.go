package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"github.com/mattetti/filebuffer"
)

func getRoot(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "Hello World!\n")
}

func main() {
	http.HandleFunc("/", getRoot)
	http.HandleFunc("/watch", getVideo)

	http.ListenAndServe(":3333", nil)
}

var downloadBuffer sync.Map

type activeTransfer struct {
	data     *filebuffer.Buffer
	progress chan bool
}

func getVideo(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("v")
	fmt.Println("Processing request for " + id)
	// Set filename hint for browsers
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment;filename="%s.mp4"`, id))

	// Check for active download buffer entry
	entry, existing := downloadBuffer.Load(id)

	if existing { // read from buffer
		videoBuffer := entry.(*activeTransfer)
		fmt.Println("- Using existing buffer")
		scanner := bufio.NewScanner(videoBuffer.data)
		scanner.Split(bufio.ScanBytes)

		for <-videoBuffer.progress {
			fmt.Println("- Reading data from buffer...")
			for scanner.Scan() {
				m := scanner.Bytes()
				w.Write(m) // Write to client
			}
		}
		fmt.Println("- Done using buffer")
		return
	}

	// Attempt to load from file cache
	fmt.Println("- Checking file cache...")
	reader := getVideoFileReader(id)
	if reader != nil {
		scanner := bufio.NewScanner(reader)
		scanner.Split(bufio.ScanBytes)
		for scanner.Scan() {
			m := scanner.Bytes()
			w.Write(m) // Write to client
		}
		fmt.Println("- Done using cached file")
		return
	} else {
		fmt.Println("- No cached file")
	}

	// No file, load into cache
	entry, existing = downloadBuffer.LoadOrStore(id, &activeTransfer{
		data:     filebuffer.New(nil),
		progress: make(chan bool),
	})
	videoBuffer := entry.(*activeTransfer)

	// We must check existing again since another request may have raced us
	if existing {
		fmt.Println("- Using existing buffer")
		scanner := bufio.NewScanner(videoBuffer.data)
		scanner.Split(bufio.ScanBytes)

		for <-videoBuffer.progress {
			for scanner.Scan() {
				m := scanner.Bytes()
				fmt.Println("- Reading new data...")
				w.Write(m) // Write to client
			}
		}
		fmt.Println("- Done using buffer")
		return
	} else {
		defer downloadBuffer.Delete(id)
		defer videoBuffer.data.Close()
		defer close(videoBuffer.progress)

		fmt.Println("Starting yt-dlp")
		cmd := exec.Command("yt-dlp", "https://www.youtube.com/watch?v="+id, "-o", "-", "--merge-output-format", "mp4")

		stdout, _ := cmd.StdoutPipe()
		cmd.Start()

		scanner := bufio.NewScanner(stdout)
		scanner.Split(bufio.ScanBytes)

		// File writer
		fmt.Println("- Opening for writing")
		file, _ := os.Create("/tmp/" + id + ".mp4")
		for scanner.Scan() {
			m := scanner.Bytes()
			w.Write(m)                // Write to first client
			videoBuffer.data.Write(m) // Write to buffer
			select {                  // Notify of progress
			case videoBuffer.progress <- true:
			default:
			}
			if file != nil {
				file.Write(m) // Write to file
			}
		}

		if err := cmd.Wait(); err != nil {
			fmt.Println("Error running yt-dlp:", err)
			file.Close()
			os.Remove(file.Name())
		} else {
			file.Close()
		}
	}
}

// Returns reader for video file by ID or nil
func getVideoFileReader(id string) io.Reader {
	file, err := os.Open("/tmp/" + id + ".mp4")
	if err != nil {
		fmt.Println("Error opening file:", err)
		return nil
	}
	fmt.Println("Reading /tmp/" + id + ".mp4")
	return file
}
